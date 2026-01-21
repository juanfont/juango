package auth

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/sessions"
	"github.com/juanfont/juango/types"
	"github.com/rs/zerolog/log"
)

// ContextKey is a custom type for context keys to avoid collisions.
type ContextKey string

const (
	// ContextKeyUser is the context key for the authenticated user.
	ContextKeyUser ContextKey = "user"
	// ContextKeyImpersonationState is the context key for impersonation state.
	ContextKeyImpersonationState ContextKey = "impersonation_state"
	// ContextKeyOriginalAdminID is the context key for the original admin ID.
	ContextKeyOriginalAdminID ContextKey = "original_admin_id"
)

// SessionMiddleware provides session-based authentication middleware.
type SessionMiddleware struct {
	sessionStore     sessions.Store
	cookieName       string
	userStore        UserStore
	auditLogger      AuditLogger
	adminModeTimeout time.Duration
}

// NewSessionMiddleware creates a new session middleware.
func NewSessionMiddleware(
	sessionStore sessions.Store,
	cookieName string,
	userStore UserStore,
	auditLogger AuditLogger,
	adminModeTimeout time.Duration,
) *SessionMiddleware {
	return &SessionMiddleware{
		sessionStore:     sessionStore,
		cookieName:       cookieName,
		userStore:        userStore,
		auditLogger:      auditLogger,
		adminModeTimeout: adminModeTimeout,
	}
}

// Authenticate validates the session and returns the user, or an error.
func (m *SessionMiddleware) Authenticate(r *http.Request) (*types.User, error) {
	session, err := m.sessionStore.Get(r, m.cookieName)
	if err != nil {
		return nil, types.NewHTTPError(http.StatusInternalServerError, "Failed to get session", err)
	}

	logged, ok := session.Values["logged"].(bool)
	if !ok || !logged {
		log.Warn().
			Str("path", r.URL.Path).
			Msg("Authentication required")
		return nil, types.NewHTTPError(http.StatusUnauthorized, "Authentication required", nil)
	}

	// Check if impersonation is active and handle expiration
	if impState, ok := session.Values["impersonation_state"].(types.ImpersonationState); ok && impState.Enabled {
		if impState.IsExpired(m.adminModeTimeout) {
			log.Warn().
				Str("admin_id", impState.OriginalAdminID.String()).
				Str("target_user_id", impState.TargetUserID.String()).
				Msg("Impersonation session expired, restoring original user")

			if originalUserIDStr, ok := session.Values["original_user_id"].(string); ok {
				session.Values["user_id"] = originalUserIDStr
				delete(session.Values, "impersonation_state")
				delete(session.Values, "original_user_id")
				session.Save(r, nil)

				// Log expiration
				if m.auditLogger != nil {
					ctx := r.Context()
					auditLog := types.NewAuditLog(
						&types.NullUUID{UUID: impState.OriginalAdminID, Valid: true},
						types.ActionImpersonationExpired,
						types.ResourceTypeUser,
						impState.TargetUserID.String(),
					).WithChanges(map[string]interface{}{
						"target_user_id":    impState.TargetUserID.String(),
						"target_user_email": impState.TargetUserEmail,
						"reason":            impState.Reason,
						"duration":          impState.Duration().String(),
					})
					m.auditLogger.CreateAuditLog(ctx, auditLog)
				}
			}

			return nil, types.NewHTTPError(http.StatusUnauthorized, "Impersonation session expired", nil)
		}
	}

	userIDStr, ok := session.Values["user_id"].(string)
	if !ok {
		return nil, types.NewHTTPError(http.StatusUnauthorized, "Invalid session", nil)
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, types.NewHTTPError(http.StatusUnauthorized, "Invalid user ID in session", nil)
	}

	user, err := m.userStore.GetUserByID(r.Context(), userID)
	if err != nil {
		return nil, types.NewHTTPError(http.StatusUnauthorized, "User not found", err)
	}

	return user, nil
}

// RequireAuth returns middleware that requires authentication.
func (m *SessionMiddleware) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := m.Authenticate(r)
		if err != nil {
			types.WriteHTTPError(w, err)
			return
		}

		ctx := context.WithValue(r.Context(), ContextKeyUser, user)

		// Add impersonation state to context if active
		session, _ := m.sessionStore.Get(r, m.cookieName)
		if session != nil {
			if impState, ok := session.Values["impersonation_state"].(types.ImpersonationState); ok && impState.Enabled {
				if !impState.IsExpired(m.adminModeTimeout) {
					ctx = context.WithValue(ctx, ContextKeyImpersonationState, impState)
					ctx = context.WithValue(ctx, ContextKeyOriginalAdminID, impState.OriginalAdminID)
				}
			}
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// RequireAuthHandler wraps an http.Handler with authentication.
// Redirects to /login on authentication failure.
func (m *SessionMiddleware) RequireAuthHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := m.Authenticate(r)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		ctx := context.WithValue(r.Context(), ContextKeyUser, user)

		session, _ := m.sessionStore.Get(r, m.cookieName)
		if session != nil {
			if impState, ok := session.Values["impersonation_state"].(types.ImpersonationState); ok && impState.Enabled {
				if !impState.IsExpired(m.adminModeTimeout) {
					ctx = context.WithValue(ctx, ContextKeyImpersonationState, impState)
					ctx = context.WithValue(ctx, ContextKeyOriginalAdminID, impState.OriginalAdminID)
				}
			}
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAdmin returns middleware that requires admin privileges.
func (m *SessionMiddleware) RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := r.Context().Value(ContextKeyUser).(*types.User)
		if !user.IsAdmin {
			log.Error().
				Str("user_id", user.ID.String()).
				Str("email", user.Email).
				Str("path", r.URL.Path).
				Msg("User is not an admin")
			types.WriteHTTPError(w, types.NewHTTPError(http.StatusForbidden, "Admin privileges required", nil))
			return
		}
		next.ServeHTTP(w, r)
	}
}

// RequireAdminMode returns middleware that requires admin mode to be enabled.
func (m *SessionMiddleware) RequireAdminMode(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := r.Context().Value(ContextKeyUser).(*types.User)
		if !user.IsAdmin {
			log.Error().
				Str("user_id", user.ID.String()).
				Str("email", user.Email).
				Str("path", r.URL.Path).
				Msg("User is not an admin")
			types.WriteHTTPError(w, types.NewHTTPError(http.StatusForbidden, "Admin privileges required", nil))
			return
		}

		session, err := m.sessionStore.Get(r, m.cookieName)
		if err != nil {
			types.WriteHTTPError(w, types.NewHTTPError(http.StatusInternalServerError, "Failed to get session", err))
			return
		}

		adminState, ok := session.Values["admin_mode"].(types.AdminModeState)
		if !ok || !adminState.Enabled {
			log.Error().
				Str("user_id", user.ID.String()).
				Str("email", user.Email).
				Str("path", r.URL.Path).
				Msg("Admin mode must be enabled to perform this action")
			types.WriteHTTPError(w, types.NewHTTPError(http.StatusForbidden, "Admin mode must be enabled to perform this action", nil))
			return
		}

		// Check if admin mode has expired
		if adminState.IsExpired(m.adminModeTimeout) {
			delete(session.Values, "admin_mode")
			session.Save(r, w)
			types.WriteHTTPError(w, types.NewHTTPError(http.StatusForbidden, "Admin mode session expired. Please re-enable admin mode.", nil))
			return
		}

		next.ServeHTTP(w, r)
	}
}

// GetUserFromContext retrieves the user from the request context.
func GetUserFromContext(ctx context.Context) *types.User {
	user, ok := ctx.Value(ContextKeyUser).(*types.User)
	if !ok {
		return nil
	}
	return user
}

// GetActorIDForAudit returns the correct user ID for audit logging.
// If impersonation is active, it returns the admin's ID.
func GetActorIDForAudit(ctx context.Context) uuid.UUID {
	if originalAdminID, ok := ctx.Value(ContextKeyOriginalAdminID).(uuid.UUID); ok && originalAdminID != uuid.Nil {
		return originalAdminID
	}

	if user, ok := ctx.Value(ContextKeyUser).(*types.User); ok {
		return user.ID
	}

	return uuid.Nil
}

// GetImpersonationContext returns impersonation details if active.
func GetImpersonationContext(ctx context.Context) (uuid.UUID, string, bool) {
	if impState, ok := ctx.Value(ContextKeyImpersonationState).(types.ImpersonationState); ok {
		return impState.TargetUserID, impState.TargetUserEmail, true
	}
	return uuid.Nil, "", false
}

// NewAuditLogWithContext creates an audit log with automatic impersonation handling.
func NewAuditLogWithContext(
	ctx context.Context,
	action string,
	resourceType string,
	resourceID string,
) *types.AuditLog {
	actorID := GetActorIDForAudit(ctx)
	var actorIDPtr *types.NullUUID
	if actorID != uuid.Nil {
		actorIDPtr = &types.NullUUID{UUID: actorID, Valid: true}
	}
	auditLog := types.NewAuditLog(actorIDPtr, action, resourceType, resourceID)

	// Add impersonation metadata if active
	if impersonatedUserID, impersonatedUserEmail, isImpersonating := GetImpersonationContext(ctx); isImpersonating {
		currentUser := ctx.Value(ContextKeyUser).(*types.User)

		existingChanges := make(map[string]interface{})
		existingChanges["_impersonation"] = map[string]interface{}{
			"impersonated_user_id":    impersonatedUserID.String(),
			"impersonated_user_email": impersonatedUserEmail,
			"impersonated_user_name":  currentUser.DisplayName,
			"performed_by_admin_id":   actorID.String(),
		}
		auditLog = auditLog.WithChanges(existingChanges)
	}

	return auditLog
}

// GetClientIP extracts the client IP address from the request.
func GetClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for proxied requests)
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[0])
	}

	// Check X-Real-IP header
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if host, _, err := net.SplitHostPort(ip); err == nil {
		return host
	}
	return ip
}
