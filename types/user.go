// Package types provides common types used by juango applications.
package types

import (
	"database/sql"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
)

// User represents an application user.
type User struct {
	ID                 uuid.UUID      `db:"id" json:"id"`
	Email              string         `db:"email" json:"email"`
	Name               string         `db:"name" json:"name"`
	LastLogin          *time.Time     `db:"last_login" json:"last_login,omitempty"`
	ProviderIdentifier sql.NullString `db:"provider_identifier" json:"-"`
	DisplayName        string         `db:"display_name" json:"display_name"`
	ProfilePicURL      string         `db:"profile_pic_url" json:"profile_pic_url"`
	IsAdmin            bool           `db:"is_admin" json:"is_admin"`

	CreatedAt  time.Time    `db:"created_at" json:"created_at"`
	ModifiedAt time.Time    `db:"modified_at" json:"modified_at"`
	DeletedAt  sql.NullTime `db:"deleted_at" json:"deleted_at,omitempty"`
}

// SessionResponse represents the response from the session check API.
type SessionResponse struct {
	Authenticated bool                `json:"authenticated"`
	User          *User               `json:"user,omitempty"`
	Reason        string              `json:"reason,omitempty"`
	Impersonation *ImpersonationState `json:"impersonation,omitempty"`
}

// FromClaim updates a User from OIDC claims.
// All fields will be updated, except for the ID.
func (u *User) FromClaim(claims *OIDCClaims) {
	u.Name = claims.Username

	if claims.EmailVerified {
		_, err := mail.ParseAddress(claims.Email)
		if err == nil {
			u.Email = claims.Email
		}
	}

	// Get provider identifier
	identifier := claims.Identifier()
	// Ensure provider identifier always has a leading slash for backward compatibility
	if claims.Iss == "" && !strings.HasPrefix(identifier, "/") {
		identifier = "/" + identifier
	}
	u.ProviderIdentifier = sql.NullString{String: identifier, Valid: true}
	u.DisplayName = claims.Name
	u.ProfilePicURL = claims.ProfilePictureURL
}

// IsActive returns true if the user is not soft-deleted.
func (u *User) IsActive() bool {
	return !u.DeletedAt.Valid
}
