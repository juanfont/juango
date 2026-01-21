package types

import "time"

// AdminModeState represents the current state of admin mode for a user session.
type AdminModeState struct {
	Enabled   bool      `json:"enabled"`
	Since     time.Time `json:"since"`
	Reason    string    `json:"reason"`
	IPAddress string    `json:"ip_address"`
}

// AdminModeRequest is the request body for enabling admin mode.
type AdminModeRequest struct {
	Reason string `json:"reason"`
}

// AdminModeStatusResponse is the response for the admin mode status endpoint.
type AdminModeStatusResponse struct {
	IsAdmin   bool            `json:"is_admin"`
	AdminMode *AdminModeState `json:"admin_mode,omitempty"`
}

// AdminModeEnableResponse is the response for enabling admin mode.
type AdminModeEnableResponse struct {
	Message string          `json:"message"`
	State   *AdminModeState `json:"state"`
}

// AdminModeDisableResponse is the response for disabling admin mode.
type AdminModeDisableResponse struct {
	Message string `json:"message"`
}

// IsExpired checks if the admin mode session has expired.
func (a *AdminModeState) IsExpired(timeout time.Duration) bool {
	if !a.Enabled {
		return true
	}
	return time.Since(a.Since) > timeout
}

// Duration returns how long admin mode has been active.
func (a *AdminModeState) Duration() time.Duration {
	return time.Since(a.Since)
}
