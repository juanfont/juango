package types

import (
	"time"

	"github.com/google/uuid"
)

// ImpersonationState represents the current state of user impersonation for an admin session.
type ImpersonationState struct {
	Enabled         bool      `json:"enabled"`
	Since           time.Time `json:"since"`
	Reason          string    `json:"reason"`
	TargetUserID    uuid.UUID `json:"target_user_id"`
	TargetUserEmail string    `json:"target_user_email"`
	TargetUserName  string    `json:"target_user_name"`
	OriginalAdminID uuid.UUID `json:"original_admin_id"`
	IPAddress       string    `json:"ip_address"`
}

// ImpersonationStartRequest is the request body for starting impersonation.
type ImpersonationStartRequest struct {
	TargetUserID string `json:"target_user_id"`
	Reason       string `json:"reason"`
}

// ImpersonationStatusResponse is the response for the impersonation status endpoint.
type ImpersonationStatusResponse struct {
	Active        bool                `json:"active"`
	Impersonation *ImpersonationState `json:"impersonation,omitempty"`
}

// ImpersonationStartResponse is the response for starting impersonation.
type ImpersonationStartResponse struct {
	Message       string              `json:"message"`
	Impersonation *ImpersonationState `json:"impersonation"`
}

// ImpersonationStopResponse is the response for stopping impersonation.
type ImpersonationStopResponse struct {
	Message string `json:"message"`
}

// IsExpired checks if the impersonation session has expired.
func (i *ImpersonationState) IsExpired(timeout time.Duration) bool {
	if !i.Enabled {
		return true
	}
	return time.Since(i.Since) > timeout
}

// Duration returns how long impersonation has been active.
func (i *ImpersonationState) Duration() time.Duration {
	return time.Since(i.Since)
}
