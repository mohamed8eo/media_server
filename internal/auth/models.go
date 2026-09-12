package auth

import "time"

type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=72,strongpassword"`
}
type LogInRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

type AuthResponse struct {
	UserID string `json:"id"`
	// Token        string    `json:"token"`
	// RefreshToken string    `json:"refresh_token,omitempty"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token,omitempty"`
}

type ResetPasswordRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=72,strongpassword"`
}
