package database

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"time"

	"mediaserver/internal/models"

	"github.com/google/uuid"
)

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func (s *service) StoreRefreshToken(userID uuid.UUID, token string, expiresAt time.Time) (uuid.UUID, error) {
	id := uuid.New()
	hashedToken := hashToken(token)
	_, err := s.db.Exec(
		`INSERT INTO refresh_tokens (id, user_id, token, expires_at, revoked) VALUES (?, ?, ?, ?, 0)`,
		id.String(), userID.String(), hashedToken, expiresAt.UTC(),
	)
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func (s *service) GetRefreshToken(token string) (*models.RefreshToken, error) {
	// Prefer hashed lookup; fall back to plaintext for tokens stored before hashing.
	rt, err := s.lookupRefreshToken(hashToken(token))
	if err == nil {
		return rt, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}
	return s.lookupRefreshToken(token)
}

func (s *service) lookupRefreshToken(tokenValue string) (*models.RefreshToken, error) {
	row := s.db.QueryRow(
		`SELECT id, user_id, token, expires_at, revoked, created_at FROM refresh_tokens WHERE token = ?`,
		tokenValue,
	)

	var rt models.RefreshToken
	var idStr, userIDStr string
	err := row.Scan(&idStr, &userIDStr, &rt.Token, &rt.ExpiresAt, &rt.Revoked, &rt.CreatedAt)
	if err != nil {
		return nil, err
	}

	rt.ID, _ = uuid.Parse(idStr)
	rt.UserID, _ = uuid.Parse(userIDStr)
	return &rt, nil
}

func (s *service) RevokeRefreshToken(token string) error {
	hashedToken := hashToken(token)
	res, err := s.db.Exec(
		`UPDATE refresh_tokens SET revoked = 1 WHERE token = ?`,
		hashedToken,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return nil
	}
	// Fallback for plaintext rows stored before hashing.
	_, err = s.db.Exec(
		`UPDATE refresh_tokens SET revoked = 1 WHERE token = ?`,
		token,
	)
	return err
}

func (s *service) RevokeAllUserRefreshTokens(userID uuid.UUID) error {
	_, err := s.db.Exec(
		`UPDATE refresh_tokens SET revoked = 1 WHERE user_id = ?`,
		userID.String(),
	)
	return err
}
