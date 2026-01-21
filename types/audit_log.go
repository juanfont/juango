package types

import (
	"database/sql"
	"database/sql/driver"
	"time"

	"github.com/google/uuid"
)

// NullUUID represents a UUID that may be null.
type NullUUID struct {
	UUID  uuid.UUID
	Valid bool
}

// Scan implements the sql.Scanner interface.
func (n *NullUUID) Scan(value interface{}) error {
	if value == nil {
		n.UUID, n.Valid = uuid.UUID{}, false
		return nil
	}
	n.Valid = true
	switch v := value.(type) {
	case string:
		var err error
		n.UUID, err = uuid.Parse(v)
		return err
	case []byte:
		var err error
		n.UUID, err = uuid.Parse(string(v))
		return err
	}
	return nil
}

// Value implements the driver.Valuer interface.
func (n NullUUID) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.UUID.String(), nil
}

// AuditLog represents a log entry for tracking changes in the system.
type AuditLog struct {
	ID           int64          `db:"id" json:"id"`
	Timestamp    time.Time      `db:"timestamp" json:"timestamp"`
	ActorUserID  NullUUID       `db:"actor_user_id" json:"actor_user_id"`
	Action       string         `db:"action" json:"action"`
	ResourceType string         `db:"resource_type" json:"resource_type"`
	ResourceID   string         `db:"resource_id" json:"resource_id"`
	Changes      JSONMap        `db:"changes" json:"changes"`
	IPAddress    sql.NullString `db:"ip_address" json:"ip_address,omitempty"`
	UserAgent    sql.NullString `db:"user_agent" json:"user_agent,omitempty"`
}

// Audit log action constants.
const (
	// User actions
	ActionUserCreated          = "user.created"
	ActionUserUpdated          = "user.updated"
	ActionUserDeactivated      = "user.deactivated"
	ActionUserReactivated      = "user.reactivated"
	ActionUserLoggedIn         = "user.logged_in"
	ActionUserLoggedOut        = "user.logged_out"
	ActionAdminModeEnabled     = "user.admin_mode_enabled"
	ActionAdminModeDisabled    = "user.admin_mode_disabled"
	ActionAdminModeExpired     = "user.admin_mode_expired"
	ActionImpersonationStarted = "user.impersonation_started"
	ActionImpersonationStopped = "user.impersonation_stopped"
	ActionImpersonationExpired = "user.impersonation_expired"

	// Task actions
	ActionTaskCreated   = "task.created"
	ActionTaskStarted   = "task.started"
	ActionTaskCompleted = "task.completed"
	ActionTaskFailed    = "task.failed"
)

// Resource types for audit logging.
const (
	ResourceTypeUser = "user"
	ResourceTypeTask = "task"
)

// NewAuditLog creates a new audit log entry with common fields.
func NewAuditLog(actorUserID *NullUUID, action, resourceType, resourceID string) *AuditLog {
	log := &AuditLog{
		Timestamp:    time.Now(),
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Changes:      make(JSONMap),
	}

	if actorUserID != nil && actorUserID.Valid {
		log.ActorUserID = *actorUserID
	}

	return log
}

// WithChanges adds change details to the audit log.
func (a *AuditLog) WithChanges(changes map[string]interface{}) *AuditLog {
	if a.Changes == nil {
		a.Changes = make(map[string]interface{})
	}
	for k, v := range changes {
		a.Changes[k] = v
	}
	return a
}

// WithBeforeAfter adds before/after state to the audit log.
func (a *AuditLog) WithBeforeAfter(before, after interface{}) *AuditLog {
	if a.Changes == nil {
		a.Changes = make(map[string]interface{})
	}
	a.Changes["before"] = before
	a.Changes["after"] = after
	return a
}

// WithIPAddress adds IP address to the audit log.
func (a *AuditLog) WithIPAddress(ip string) *AuditLog {
	if ip != "" {
		a.IPAddress = sql.NullString{String: ip, Valid: true}
	}
	return a
}

// WithUserAgent adds user agent to the audit log.
func (a *AuditLog) WithUserAgent(ua string) *AuditLog {
	if ua != "" {
		a.UserAgent = sql.NullString{String: ua, Valid: true}
	}
	return a
}

// AddDetail adds a key-value detail to the changes map.
func (a *AuditLog) AddDetail(key string, value interface{}) *AuditLog {
	if a.Changes == nil {
		a.Changes = make(JSONMap)
	}
	a.Changes[key] = value
	return a
}
