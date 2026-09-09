package auth

import (
	"testing"

	"github.com/google/uuid"
)

func TestPasswordHashing(t *testing.T) {
	password := "SecurePassword123!"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	if !CheckPassword(password, hash) {
		t.Error("expected password to match hash")
	}

	if CheckPassword("WrongPassword123!", hash) {
		t.Error("expected wrong password to fail check")
	}
}

func TestJWTGenerationAndValidation(t *testing.T) {
	jwtService := NewJWT("test-secret-key")
	userID := uuid.New()

	accessToken, _, err := jwtService.GenerateAccessToken(userID)
	if err != nil {
		t.Fatalf("failed to generate access token: %v", err)
	}

	claims, err := jwtService.ValidateToken(accessToken)
	if err != nil {
		t.Fatalf("failed to validate access token: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("expected user ID %v; got %v", userID, claims.UserID)
	}

	if claims.Type != "access" {
		t.Errorf("expected claim type 'access'; got '%s'", claims.Type)
	}

	refreshToken, _, err := jwtService.GenerateRefreshToken(userID)
	if err != nil {
		t.Fatalf("failed to generate refresh token: %v", err)
	}

	refreshClaims, err := jwtService.ValidateToken(refreshToken)
	if err != nil {
		t.Fatalf("failed to validate refresh token: %v", err)
	}

	if refreshClaims.UserID != userID {
		t.Errorf("expected user ID %v; got %v", userID, refreshClaims.UserID)
	}

	if refreshClaims.Type != "refresh" {
		t.Errorf("expected claim type 'refresh'; got '%s'", refreshClaims.Type)
	}
}
