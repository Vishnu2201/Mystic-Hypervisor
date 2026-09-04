package auth

import (
	"context"
	"time"
)

// User represents a system user entity.
type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Session represents an active authenticated session.
type Session struct {
	Token     string    `json:"token"`
	UserID    string    `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

// AuthService defines boundary operations for user authentication.
type AuthService interface {
	Authenticate(ctx context.Context, username, password string) (*Session, error)
	ValidateToken(ctx context.Context, token string) (*User, error)
	RevokeToken(ctx context.Context, token string) error
}
