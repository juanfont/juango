package types

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// NotificationType represents the type of notification.
type NotificationType string

const (
	NotificationTypeInfo    NotificationType = "info"
	NotificationTypeWarning NotificationType = "warning"
	NotificationTypeError   NotificationType = "error"
	NotificationTypeSuccess NotificationType = "success"
)

// Notification represents a user notification.
type Notification struct {
	ID        uuid.UUID        `db:"id" json:"id"`
	UserID    uuid.UUID        `db:"user_id" json:"user_id"`
	Type      NotificationType `db:"type" json:"type"`
	Title     string           `db:"title" json:"title"`
	Message   string           `db:"message" json:"message"`
	Link      sql.NullString   `db:"link" json:"link,omitempty"`
	Read      bool             `db:"read" json:"read"`
	ReadAt    sql.NullTime     `db:"read_at" json:"read_at,omitempty"`
	CreatedAt time.Time        `db:"created_at" json:"created_at"`
}

// NotificationCreateRequest is the request body for creating a notification.
type NotificationCreateRequest struct {
	UserID  uuid.UUID        `json:"user_id"`
	Type    NotificationType `json:"type"`
	Title   string           `json:"title"`
	Message string           `json:"message"`
	Link    string           `json:"link,omitempty"`
}

// NotificationListResponse is the response for listing notifications.
type NotificationListResponse struct {
	Notifications []Notification `json:"notifications"`
	UnreadCount   int            `json:"unread_count"`
}

// UnreadCountResponse is the response for getting unread notification count.
type UnreadCountResponse struct {
	Count int `json:"count"`
}
