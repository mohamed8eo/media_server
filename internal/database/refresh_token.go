package database

import (
	"time"

	"github.com/google/uuid"
	"mediaserver/internal/models"
)

func (s *service) StoreRefreshToken(userID uuid.UUID, token string, expiresAt time.Time) (uuid.UUID, error) {
	id := uuid.New()
	_, err := s.db.Exec(
		`INSERT INTO refresh_tokens (id, user_id, token, expires_at, revoked) VALUES (?, ?, ?, ?, 0)`,
		id.String(), userID.String(), token, expiresAt,
	)
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func (s *service) GetRefreshToken(token string) (*models.RefreshToken, error) {
	row := s.db.QueryRow(
		`SELECT id, user_id, token, expires_at, revoked, created_at FROM refresh_tokens WHERE token = ?`,
		token,
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
	_, err := s.db.Exec(
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
