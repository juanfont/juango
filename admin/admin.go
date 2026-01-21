// Package admin provides admin mode and impersonation functionality.
package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/sessions"
	"github.com/juanfont/juango/auth"
	"github.com/juanfont/juango/types"
	"github.com/rs/zerolog/log"
)

// Handlers provides HTTP handlers for admin mode and impersonation.
type Handlers struct {
	sessionStore     sessions.Store
	cookieName       string
	userStore        auth.UserStore
	auditLogger      auth.AuditLogger
	adminModeTimeout time.Duration
}

// NewHandlers creates new admin handlers.
func NewHandlers(
	sessionStore sessions.Store,
	cookieName string,
	userStore auth.UserStore,
	auditLogger auth.AuditLogger,
	adminModeTimeout time.Duration,
) *Handlers {
	return &Handlers{
		sessionStore:     sessionStore,
		cookieName:       cookieName,
		userStore:        userStore,
		auditLogger:      auditLogger,
		adminModeTimeout: adminModeTimeout,
	}
}

// AdminModeStatusHandler handles GET /api/admin/mode/status.
func (h *Handlers) AdminModeStatusHandler(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())

	response := types.AdminModeStatusResponse{
		IsAdmin: user.IsAdmin,
	}

	if user.IsAdmin {
		session, err := h.sessionStore.Get(r, h.cookieName)
		if err == nil {
			if adminState, ok := session.Values["admin_mode"].(types.AdminModeState); ok {
				if adminState.IsExpired(h.adminModeTimeout) {
					delete(session.Values, "admin_mode")
					session.Save(r, w)
				} else {
					response.AdminMode = &adminState
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// AdminModeEnableHandler handles POST /api/admin/mode/enable.
func (h *Handlers) AdminModeEnableHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := auth.GetUserFromContext(ctx)

	var req types.AdminModeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		types.WriteHTTPError(w, types.NewHTTPError(http.StatusBadRequest, "Invalid request body", err))
		return
	}

	if strings.TrimSpace(req.Reason) == "" {
		types.WriteHTTPError(w, types.NewHTTPError(http.StatusBadRequest, "Reason is required for admin mode", nil))
		return
	}

	session, err := h.sessionStore.Get(r, h.cookieName)
	if err != nil {
		types.WriteHTTPError(w, types.NewHTTPError(http.StatusInternalServerError, "Failed to get session", err))
		return
	}

	adminState := types.AdminModeState{
		Enabled:   true,
		Since:     time.Now(),
		Reason:    strings.TrimSpace(req.Reason),
		IPAddress: auth.GetClientIP(r),
	}

	session.Values["admin_mode"] = adminState
	if err := session.Save(r, w); err != nil {
		types.WriteHTTPError(w, types.NewHTTPError(http.StatusInternalServerError, "Failed to save session", err))
		return
	}

	log.Info().
		Str("admin_id", user.ID.String()).
		Str("admin_email", user.Email).
		Str("reason", adminState.Reason).
		Str("ip", adminState.IPAddress).
		Msg("Admin mode enabled")

	if h.auditLogger != nil {
		auditLog := auth.NewAuditLogWithContext(
			ctx,
			types.ActionAdminModeEnabled,
			types.ResourceTypeUser,
			user.ID.String(),
		).WithChanges(map[string]interface{}{
			"reason":     adminState.Reason,
			"ip_address": adminState.IPAddress,
			"timeout":    h.adminModeTimeout.String(),
		}).WithIPAddress(adminState.IPAddress).WithUserAgent(r.UserAgent())

		if err := h.auditLogger.CreateAuditLog(ctx, auditLog); err != nil {
			log.Error().Err(err).Msg("Failed to create audit log for admin mode enable")
		}
	}

	response := types.AdminModeEnableResponse{
		Message: "Admin mode enabled",
		State:   &adminState,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// AdminModeDisableHandler handles POST /api/admin/mode/disable.
func (h *Handlers) AdminModeDisableHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := auth.GetUserFromContext(ctx)

	session, err := h.sessionStore.Get(r, h.cookieName)
	if err != nil {
		types.WriteHTTPError(w, types.NewHTTPError(http.StatusInternalServerError, "Failed to get session", err))
		return
	}

	var previousState types.AdminModeState
	if adminState, ok := session.Values["admin_mode"].(types.AdminModeState); ok {
		previousState = adminState
	}

	// Also stop any active impersonation
	delete(session.Values, "admin_mode")
	delete(session.Values, "impersonation_state")
	delete(session.Values, "original_user_id")

	if err := session.Save(r, w); err != nil {
		types.WriteHTTPError(w, types.NewHTTPError(http.StatusInternalServerError, "Failed to save session", err))
		return
	}

	log.Info().
		Str("admin_id", user.ID.String()).
		Str("admin_email", user.Email).
		Str("previous_reason", previousState.Reason).
		Dur("duration", previousState.Duration()).
		Str("ip", auth.GetClientIP(r)).
		Msg("Admin mode disabled")

	if h.auditLogger != nil {
		auditLog := auth.NewAuditLogWithContext(
			ctx,
			types.ActionAdminModeDisabled,
			types.ResourceTypeUser,
			user.ID.String(),
		).WithChanges(map[string]interface{}{
			"previous_reason": previousState.Reason,
			"duration":        previousState.Duration().String(),
			"ip_address":      auth.GetClientIP(r),
		}).WithIPAddress(auth.GetClientIP(r)).WithUserAgent(r.UserAgent())

		if err := h.auditLogger.CreateAuditLog(ctx, auditLog); err != nil {
			log.Error().Err(err).Msg("Failed to create audit log for admin mode disable")
		}
	}

	response := types.AdminModeDisableResponse{
		Message: "Admin mode disabled",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ImpersonationStartHandler handles POST /api/admin/impersonate/start.
func (h *Handlers) ImpersonationStartHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	adminUser := auth.GetUserFromContext(ctx)

	var req types.ImpersonationStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		types.WriteHTTPError(w, types.NewHTTPError(http.StatusBadRequest, "Invalid request body", err))
		return
	}

	if strings.TrimSpace(req.Reason) == "" {
		types.WriteHTTPError(w, types.NewHTTPError(http.StatusBadRequest, "Reason is required for impersonation", nil))
		return
	}

	targetUserID, err := uuid.Parse(req.TargetUserID)
	if err != nil || targetUserID == uuid.Nil {
		types.WriteHTTPError(w, types.NewHTTPError(http.StatusBadRequest, "Target user ID is required", err))
		return
	}

	if targetUserID == adminUser.ID {
		types.WriteHTTPError(w, types.NewHTTPError(http.StatusBadRequest, "Cannot impersonate yourself", nil))
		return
	}

	session, err := h.sessionStore.Get(r, h.cookieName)
	if err != nil {
		types.WriteHTTPError(w, types.NewHTTPError(http.StatusInternalServerError, "Failed to get session", err))
		return
	}

	if existingState, ok := session.Values["impersonation_state"].(types.ImpersonationState); ok && existingState.Enabled {
		types.WriteHTTPError(w, types.NewHTTPError(http.StatusBadRequest, "Already impersonating another user. Stop current impersonation first.", nil))
		return
	}

	targetUser, err := h.userStore.GetUserByID(ctx, targetUserID)
	if err != nil {
		types.WriteHTTPError(w, types.NewHTTPError(http.StatusNotFound, "Target user not found", err))
		return
	}

	if targetUser.IsAdmin {
		types.WriteHTTPError(w, types.NewHTTPError(http.StatusForbidden, "Cannot impersonate admin users", nil))
		return
	}

	originalAdminID := adminUser.ID

	impersonationState := types.ImpersonationState{
		Enabled:         true,
		Since:           time.Now(),
		Reason:          strings.TrimSpace(req.Reason),
		TargetUserID:    targetUser.ID,
		TargetUserEmail: targetUser.Email,
		TargetUserName:  targetUser.DisplayName,
		OriginalAdminID: originalAdminID,
		IPAddress:       auth.GetClientIP(r),
	}

	session.Values["impersonation_state"] = impersonationState
	session.Values["original_user_id"] = originalAdminID.String()
	session.Values["user_id"] = targetUser.ID.String()

	if err := session.Save(r, w); err != nil {
		types.WriteHTTPError(w, types.NewHTTPError(http.StatusInternalServerError, "Failed to save session", err))
		return
	}

	log.Info().
		Str("admin_id", originalAdminID.String()).
		Str("admin_email", adminUser.Email).
		Str("target_user_id", targetUser.ID.String()).
		Str("target_user_email", targetUser.Email).
		Str("reason", impersonationState.Reason).
		Str("ip", impersonationState.IPAddress).
		Msg("Impersonation started")

	if h.auditLogger != nil {
		auditLog := auth.NewAuditLogWithContext(
			ctx,
			types.ActionImpersonationStarted,
			types.ResourceTypeUser,
			targetUser.ID.String(),
		).WithChanges(map[string]interface{}{
			"admin_id":          originalAdminID.String(),
			"admin_email":       adminUser.Email,
			"target_user_id":    targetUser.ID.String(),
			"target_user_email": targetUser.Email,
			"target_user_name":  targetUser.DisplayName,
			"reason":            impersonationState.Reason,
			"ip_address":        impersonationState.IPAddress,
			"timeout":           h.adminModeTimeout.String(),
		}).WithIPAddress(impersonationState.IPAddress).WithUserAgent(r.UserAgent())

		if err := h.auditLogger.CreateAuditLog(ctx, auditLog); err != nil {
			log.Error().Err(err).Msg("Failed to create audit log for impersonation start")
		}
	}

	response := types.ImpersonationStartResponse{
		Message:       fmt.Sprintf("Now impersonating %s", targetUser.Email),
		Impersonation: &impersonationState,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ImpersonationStopHandler handles POST /api/admin/impersonate/stop.
func (h *Handlers) ImpersonationStopHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	session, err := h.sessionStore.Get(r, h.cookieName)
	if err != nil {
		types.WriteHTTPError(w, types.NewHTTPError(http.StatusInternalServerError, "Failed to get session", err))
		return
	}

	impersonationState, ok := session.Values["impersonation_state"].(types.ImpersonationState)
	if !ok || !impersonationState.Enabled {
		types.WriteHTTPError(w, types.NewHTTPError(http.StatusBadRequest, "Not currently impersonating", nil))
		return
	}

	originalAdminIDStr, ok := session.Values["original_user_id"].(string)
	if !ok {
		types.WriteHTTPError(w, types.NewHTTPError(http.StatusInternalServerError, "Original user ID not found in session", nil))
		return
	}

	originalAdminID, err := uuid.Parse(originalAdminIDStr)
	if err != nil {
		types.WriteHTTPError(w, types.NewHTTPError(http.StatusInternalServerError, "Invalid original user ID in session", nil))
		return
	}

	adminUser, err := h.userStore.GetUserByID(ctx, originalAdminID)
	if err != nil {
		log.Error().Err(err).Str("admin_id", originalAdminID.String()).Msg("Failed to fetch admin user during impersonation stop")
	}

	duration := impersonationState.Duration()

	session.Values["user_id"] = originalAdminIDStr
	delete(session.Values, "impersonation_state")
	delete(session.Values, "original_user_id")

	if err := session.Save(r, w); err != nil {
		types.WriteHTTPError(w, types.NewHTTPError(http.StatusInternalServerError, "Failed to save session", err))
		return
	}

	log.Info().
		Str("admin_id", originalAdminID.String()).
		Str("admin_email", func() string {
			if adminUser != nil {
				return adminUser.Email
			}
			return ""
		}()).
		Str("target_user_id", impersonationState.TargetUserID.String()).
		Str("target_user_email", impersonationState.TargetUserEmail).
		Dur("duration", duration).
		Str("ip", auth.GetClientIP(r)).
		Msg("Impersonation stopped")

	if h.auditLogger != nil {
		auditLog := auth.NewAuditLogWithContext(
			ctx,
			types.ActionImpersonationStopped,
			types.ResourceTypeUser,
			impersonationState.TargetUserID.String(),
		).WithChanges(map[string]interface{}{
			"admin_id":          originalAdminID.String(),
			"target_user_id":    impersonationState.TargetUserID.String(),
			"target_user_email": impersonationState.TargetUserEmail,
			"reason":            impersonationState.Reason,
			"duration":          duration.String(),
			"ip_address":        auth.GetClientIP(r),
		}).WithIPAddress(auth.GetClientIP(r)).WithUserAgent(r.UserAgent())

		if err := h.auditLogger.CreateAuditLog(ctx, auditLog); err != nil {
			log.Error().Err(err).Msg("Failed to create audit log for impersonation stop")
		}
	}

	response := types.ImpersonationStopResponse{
		Message: "Impersonation stopped successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ImpersonationStatusHandler handles GET /api/admin/impersonate/status.
func (h *Handlers) ImpersonationStatusHandler(w http.ResponseWriter, r *http.Request) {
	session, err := h.sessionStore.Get(r, h.cookieName)
	if err != nil {
		types.WriteHTTPError(w, types.NewHTTPError(http.StatusInternalServerError, "Failed to get session", err))
		return
	}

	response := types.ImpersonationStatusResponse{
		Active: false,
	}

	if impersonationState, ok := session.Values["impersonation_state"].(types.ImpersonationState); ok && impersonationState.Enabled {
		if impersonationState.IsExpired(h.adminModeTimeout) {
			h.stopExpiredImpersonation(w, r, session, impersonationState)
		} else {
			response.Active = true
			response.Impersonation = &impersonationState
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// stopExpiredImpersonation cleans up an expired impersonation session.
func (h *Handlers) stopExpiredImpersonation(w http.ResponseWriter, r *http.Request, session *sessions.Session, state types.ImpersonationState) {
	ctx := r.Context()

	originalAdminIDStr, ok := session.Values["original_user_id"].(string)
	if !ok {
		log.Error().Msg("Original user ID not found during expired impersonation cleanup")
		return
	}

	originalAdminID, err := uuid.Parse(originalAdminIDStr)
	if err != nil {
		log.Error().Err(err).Msg("Invalid original user ID during expired impersonation cleanup")
		return
	}

	session.Values["user_id"] = originalAdminIDStr
	delete(session.Values, "impersonation_state")
	delete(session.Values, "original_user_id")
	session.Save(r, w)

	log.Warn().
		Str("admin_id", originalAdminID.String()).
		Str("target_user_id", state.TargetUserID.String()).
		Str("target_user_email", state.TargetUserEmail).
		Dur("duration", state.Duration()).
		Msg("Impersonation session expired")

	if h.auditLogger != nil {
		auditLog := auth.NewAuditLogWithContext(
			ctx,
			types.ActionImpersonationExpired,
			types.ResourceTypeUser,
			state.TargetUserID.String(),
		).WithChanges(map[string]interface{}{
			"admin_id":          originalAdminID.String(),
			"target_user_id":    state.TargetUserID.String(),
			"target_user_email": state.TargetUserEmail,
			"reason":            state.Reason,
			"duration":          state.Duration().String(),
			"ip_address":        auth.GetClientIP(r),
		}).WithIPAddress(auth.GetClientIP(r)).WithUserAgent(r.UserAgent())

		if err := h.auditLogger.CreateAuditLog(context.Background(), auditLog); err != nil {
			log.Error().Err(err).Msg("Failed to create audit log for impersonation expiration")
		}
	}
}
