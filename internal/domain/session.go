package domain

import "time"

type SessionStatus string

const (
	SessionRunning SessionStatus = "running"
	SessionDone    SessionStatus = "done"
	SessionError   SessionStatus = "error"
)

type Session struct {
	ID         string        `json:"id"`
	UserID     string        `json:"user_id"`
	UserEmail  string        `json:"user_email,omitempty"`
	UserName   string        `json:"user_name,omitempty"`
	DeviceType string        `json:"device_type"`
	StartedAt  time.Time     `json:"started_at"`
	FinishedAt *time.Time    `json:"finished_at,omitempty"`
	TTNCount   int           `json:"ttn_count"`
	Status     SessionStatus `json:"status"`
}

type SessionTTN struct {
	ID        int64     `json:"id"`
	SessionID string    `json:"session_id"`
	TTN       string    `json:"ttn"`
	Status    string    `json:"status"`
	Message   string    `json:"message,omitempty"`
	Registry  string    `json:"registry,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
