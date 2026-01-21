// Package auth provides OIDC authentication and session management.
package auth

import (
	"cmp"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"
	"github.com/juanfont/juango/types"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
)

const (
	// OIDCCallbackPath is the default callback path for OIDC.
	OIDCCallbackPath = "/api/oidc/callback"
)

// OIDCProvider handles OIDC authentication.
type OIDCProvider struct {
	serverURL    string
	config       types.OIDCConfig
	callbackPath string

	verifier     *oidc.IDTokenVerifier
	provider     *oidc.Provider
	oauth2Config *oauth2.Config
}

// OIDCProviderConfig holds configuration for creating an OIDC provider.
type OIDCProviderConfig struct {
	ServerURL    string
	OIDCConfig   types.OIDCConfig
	CallbackPath string
}

// NewOIDCProvider creates a new OIDC provider.
func NewOIDCProvider(ctx context.Context, cfg OIDCProviderConfig) (*OIDCProvider, error) {
	if cfg.CallbackPath == "" {
		cfg.CallbackPath = OIDCCallbackPath
	}

	provider, err := oidc.NewProvider(ctx, cfg.OIDCConfig.Issuer)
	if err != nil {
		return nil, fmt.Errorf("creating OIDC provider from issuer config: %w", err)
	}

	oauth2Config := &oauth2.Config{
		ClientID:     cfg.OIDCConfig.ClientID,
		ClientSecret: cfg.OIDCConfig.ClientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL: fmt.Sprintf(
			"%s%s",
			strings.TrimSuffix(cfg.ServerURL, "/"),
			cfg.CallbackPath,
		),
		Scopes: cfg.OIDCConfig.Scopes,
	}

	// Microsoft Entra ID requires skipping signature check
	verifier := provider.Verifier(&oidc.Config{
		ClientID:                   cfg.OIDCConfig.ClientID,
		InsecureSkipSignatureCheck: strings.Contains(cfg.OIDCConfig.Issuer, "microsoft"),
	})

	return &OIDCProvider{
		serverURL:    cfg.ServerURL,
		config:       cfg.OIDCConfig,
		callbackPath: cfg.CallbackPath,
		provider:     provider,
		oauth2Config: oauth2Config,
		verifier:     verifier,
	}, nil
}

// CallbackPath returns the OIDC callback path.
func (p *OIDCProvider) CallbackPath() string {
	return p.callbackPath
}

// AuthCodeURL generates the authorization URL for the OIDC flow.
func (p *OIDCProvider) AuthCodeURL(state, nonce string) string {
	return p.oauth2Config.AuthCodeURL(state, oidc.Nonce(nonce))
}

// Exchange exchanges an authorization code for tokens.
func (p *OIDCProvider) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return p.oauth2Config.Exchange(ctx, code)
}

// VerifyIDToken verifies an ID token and returns it.
func (p *OIDCProvider) VerifyIDToken(ctx context.Context, rawIDToken string) (*oidc.IDToken, error) {
	return p.verifier.Verify(ctx, rawIDToken)
}

// UserInfo fetches user info from the OIDC provider.
func (p *OIDCProvider) UserInfo(ctx context.Context, token *oauth2.Token) (*oidc.UserInfo, error) {
	return p.provider.UserInfo(ctx, oauth2.StaticTokenSource(token))
}

// ProcessCallback handles the OIDC callback and returns claims.
func (p *OIDCProvider) ProcessCallback(ctx context.Context, code, expectedNonce string, token *oauth2.Token) (*types.OIDCClaims, error) {
	// Extract the ID Token from OAuth2 token
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("missing id token")
	}

	// Parse and verify ID Token
	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("unable to verify id token: %w", err)
	}

	if idToken.Nonce != expectedNonce {
		return nil, fmt.Errorf("nonce did not match")
	}

	var claims types.OIDCClaims
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("decoding ID token claims: %w", err)
	}

	// Fetch userinfo to supplement claims
	userinfo, err := p.provider.UserInfo(ctx, oauth2.StaticTokenSource(token))
	if err != nil {
		log.Warn().Err(err).Msg("could not get userinfo; only checking claim")
	}

	if userinfo != nil && userinfo.Subject == claims.Sub {
		claims.Email = cmp.Or(claims.Email, userinfo.Email)
		claims.EmailVerified = cmp.Or(claims.EmailVerified, types.FlexibleBoolean(userinfo.EmailVerified))

		var userinfo2 types.OIDCUserInfo
		if err := userinfo.Claims(&userinfo2); err == nil {
			claims.Username = cmp.Or(claims.Username, userinfo2.PreferredUsername)
			claims.Name = cmp.Or(claims.Name, userinfo2.Name)

			pictureURL := cmp.Or(claims.ProfilePictureURL, userinfo2.Picture)

			// Handle Microsoft Entra ID profile photos
			isMicrosoft := strings.Contains(claims.Iss, "login.microsoftonline.com")
			if isMicrosoft && pictureURL != "" {
				dataURL := fetchMicrosoftGraphPhoto(ctx, token.AccessToken, pictureURL)
				if dataURL != "" {
					claims.ProfilePictureURL = dataURL
				} else {
					claims.ProfilePictureURL = ""
				}
			} else {
				claims.ProfilePictureURL = pictureURL
			}
		}
	}

	return &claims, nil
}

// GenerateRandomState generates a secure random state string for OIDC flows.
func GenerateRandomState() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", fmt.Errorf("generating random state: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// fetchMicrosoftGraphPhoto fetches a profile photo from Microsoft Graph API.
func fetchMicrosoftGraphPhoto(ctx context.Context, accessToken, photoURL string) string {
	photoURL = strings.TrimPrefix(photoURL, "@")

	req, err := http.NewRequestWithContext(ctx, "GET", photoURL, nil)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to create request for Microsoft Graph photo")
		return ""
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to fetch Microsoft Graph photo")
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		log.Debug().Msg("User has no profile photo in Microsoft Graph")
		return ""
	}

	if resp.StatusCode != http.StatusOK {
		log.Warn().Int("status", resp.StatusCode).Msg("Microsoft Graph photo request returned non-OK status")
		return ""
	}

	photoData, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to read Microsoft Graph photo data")
		return ""
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}

	base64Data := base64.StdEncoding.EncodeToString(photoData)
	return fmt.Sprintf("data:%s;base64,%s", contentType, base64Data)
}

// UserStore is the interface for user database operations.
type UserStore interface {
	CreateOrUpdateUserFromClaim(claims *types.OIDCClaims) (*types.User, error)
	UpdateLastLogin(ctx context.Context, userID uuid.UUID) error
	GetUserByID(ctx context.Context, userID uuid.UUID) (*types.User, error)
}

// AuditLogger is the interface for audit logging.
type AuditLogger interface {
	CreateAuditLog(ctx context.Context, log *types.AuditLog) error
}

// OIDCHandlers provides HTTP handlers for OIDC authentication.
type OIDCHandlers struct {
	provider     *OIDCProvider
	sessionStore sessions.Store
	cookieName   string
	userStore    UserStore
	auditLogger  AuditLogger
}

// NewOIDCHandlers creates new OIDC handlers.
func NewOIDCHandlers(provider *OIDCProvider, sessionStore sessions.Store, cookieName string, userStore UserStore, auditLogger AuditLogger) *OIDCHandlers {
	return &OIDCHandlers{
		provider:     provider,
		sessionStore: sessionStore,
		cookieName:   cookieName,
		userStore:    userStore,
		auditLogger:  auditLogger,
	}
}

// LoginHandler redirects to the OIDC provider for authentication.
func (h *OIDCHandlers) LoginHandler(w http.ResponseWriter, r *http.Request) {
	session, err := h.sessionStore.Get(r, h.cookieName)
	if err != nil {
		types.WriteHTTPError(w, err)
		return
	}

	state, err := GenerateRandomState()
	if err != nil {
		types.WriteHTTPError(w, err)
		return
	}

	nonce, err := GenerateRandomState()
	if err != nil {
		types.WriteHTTPError(w, err)
		return
	}

	session.Values["state"] = state
	session.Values["nonce"] = nonce

	if err := session.Save(r, w); err != nil {
		types.WriteHTTPError(w, err)
		return
	}

	authURL := h.provider.AuthCodeURL(state, nonce)
	log.Debug().Str("url", authURL).Msg("Redirecting to OIDC provider")
	http.Redirect(w, r, authURL, http.StatusFound)
}

// CallbackHandler handles the OIDC callback.
func (h *OIDCHandlers) CallbackHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	session, err := h.sessionStore.Get(r, h.cookieName)
	if err != nil {
		types.WriteHTTPError(w, err)
		return
	}

	expectedState, ok := session.Values["state"].(string)
	if !ok || r.URL.Query().Get("state") != expectedState {
		types.WriteHTTPError(w, types.NewHTTPError(http.StatusBadRequest, "Invalid state parameter", nil))
		return
	}

	expectedNonce, ok := session.Values["nonce"].(string)
	if !ok {
		types.WriteHTTPError(w, types.NewHTTPError(http.StatusBadRequest, "Nonce not found", nil))
		return
	}

	// Clear state and nonce to prevent replay attacks
	delete(session.Values, "state")
	delete(session.Values, "nonce")
	if err := session.Save(r, w); err != nil {
		types.WriteHTTPError(w, err)
		return
	}

	// Exchange code for token
	token, err := h.provider.Exchange(ctx, r.URL.Query().Get("code"))
	if err != nil {
		types.WriteHTTPError(w, types.NewHTTPError(http.StatusInternalServerError, "Unable to exchange authorization code", err))
		return
	}

	// Process callback and get claims
	claims, err := h.provider.ProcessCallback(ctx, r.URL.Query().Get("code"), expectedNonce, token)
	if err != nil {
		types.WriteHTTPError(w, types.NewHTTPError(http.StatusInternalServerError, "Failed to process OIDC callback", err))
		return
	}

	// Create or update user
	user, err := h.userStore.CreateOrUpdateUserFromClaim(claims)
	if err != nil {
		types.WriteHTTPError(w, err)
		return
	}

	if err := h.userStore.UpdateLastLogin(ctx, user.ID); err != nil {
		types.WriteHTTPError(w, err)
		return
	}

	// Create audit log
	if h.auditLogger != nil {
		auditLog := types.NewAuditLog(
			&types.NullUUID{UUID: user.ID, Valid: true},
			types.ActionUserLoggedIn,
			types.ResourceTypeUser,
			user.ID.String(),
		).WithChanges(map[string]interface{}{
			"email":        user.Email,
			"display_name": user.DisplayName,
		}).WithIPAddress(GetClientIP(r)).WithUserAgent(r.UserAgent())

		if err := h.auditLogger.CreateAuditLog(ctx, auditLog); err != nil {
			log.Error().Err(err).Msg("Failed to create audit log for login")
		}
	}

	// Save session
	session, err = h.sessionStore.Get(r, h.cookieName)
	if err != nil {
		types.WriteHTTPError(w, err)
		return
	}

	session.Values["logged"] = true
	session.Values["user_id"] = user.ID.String()

	if err := session.Save(r, w); err != nil {
		types.WriteHTTPError(w, err)
		return
	}

	http.Redirect(w, r, "/", http.StatusFound)
}

// LogoutHandler handles logout.
func (h *OIDCHandlers) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	session, err := h.sessionStore.Get(r, h.cookieName)
	if err != nil {
		types.WriteHTTPError(w, err)
		return
	}

	// Get user ID for audit log
	var userID uuid.UUID
	if idStr, ok := session.Values["user_id"].(string); ok {
		userID, _ = uuid.Parse(idStr)
	}

	// Create audit log
	if h.auditLogger != nil && userID != uuid.Nil {
		auditLog := types.NewAuditLog(
			&types.NullUUID{UUID: userID, Valid: true},
			types.ActionUserLoggedOut,
			types.ResourceTypeUser,
			userID.String(),
		).WithIPAddress(GetClientIP(r)).WithUserAgent(r.UserAgent())

		if err := h.auditLogger.CreateAuditLog(ctx, auditLog); err != nil {
			log.Error().Err(err).Msg("Failed to create audit log for logout")
		}
	}

	// Clear session
	delete(session.Values, "logged")
	delete(session.Values, "user_id")
	delete(session.Values, "admin_mode")
	delete(session.Values, "impersonation_state")
	delete(session.Values, "original_user_id")

	if err := session.Save(r, w); err != nil {
		types.WriteHTTPError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Logged out successfully",
	})
}

// SessionCheckHandler checks the current session status.
func (h *OIDCHandlers) SessionCheckHandler(w http.ResponseWriter, r *http.Request) {
	session, err := h.sessionStore.Get(r, h.cookieName)
	if err != nil {
		types.WriteHTTPError(w, types.NewHTTPError(http.StatusInternalServerError, "Failed to get session", err))
		return
	}

	logged, ok := session.Values["logged"].(bool)
	if !ok || !logged {
		reason := "not_authenticated"
		if session.IsNew {
			reason = "session_expired"
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(&types.SessionResponse{
			Authenticated: false,
			Reason:        reason,
		})
		return
	}

	userIDStr, ok := session.Values["user_id"].(string)
	if !ok {
		delete(session.Values, "logged")
		delete(session.Values, "user_id")
		session.Save(r, w)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(&types.SessionResponse{
			Authenticated: false,
			Reason:        "session_corrupted",
		})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		delete(session.Values, "logged")
		delete(session.Values, "user_id")
		session.Save(r, w)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(&types.SessionResponse{
			Authenticated: false,
			Reason:        "session_corrupted",
		})
		return
	}

	user, err := h.userStore.GetUserByID(r.Context(), userID)
	if err != nil {
		delete(session.Values, "logged")
		delete(session.Values, "user_id")
		session.Save(r, w)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(&types.SessionResponse{
			Authenticated: false,
			Reason:        "user_not_found",
		})
		return
	}

	response := &types.SessionResponse{
		Authenticated: true,
		User:          user,
	}

	// Include impersonation state if active
	if impState, ok := session.Values["impersonation_state"].(types.ImpersonationState); ok && impState.Enabled {
		response.Impersonation = &impState
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
