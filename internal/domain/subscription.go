package domain

import "time"

type Subscription struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	GrantedBy string    `json:"granted_by"`
	StartsAt  time.Time `json:"starts_at"`
	EndsAt    time.Time `json:"ends_at"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Subscription) IsActive() bool {
	now := time.Now()
	return !now.Before(s.StartsAt) && now.Before(s.EndsAt.Add(24*time.Hour))
}

type SubscriptionStatus struct {
	Active   bool      `json:"active"`
	StartsAt time.Time `json:"starts_at,omitempty"`
	EndsAt   time.Time `json:"ends_at,omitempty"`
}
