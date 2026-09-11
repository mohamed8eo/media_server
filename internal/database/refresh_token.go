package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"time"

	"mediaserver/internal/database/sqlc"
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
	ctx := context.Background()
	err := s.queries.StoreRefreshToken(ctx, sqlc.StoreRefreshTokenParams{
		ID:        id.String(),
		UserID:    userID.String(),
		Token:     hashedToken,
		ExpiresAt: expiresAt.UTC(),
	})
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func (s *service) GetRefreshToken(token string) (*models.RefreshToken, error) {
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
	ctx := context.Background()
	dbRt, err := s.queries.GetRefreshTokenByToken(ctx, tokenValue)
	if err != nil {
		return nil, err
	}

	uid, _ := uuid.Parse(dbRt.ID)
	userID, _ := uuid.Parse(dbRt.UserID)
	var createdAt time.Time
	if dbRt.CreatedAt.Valid {
		createdAt = dbRt.CreatedAt.Time
	}

	return &models.RefreshToken{
		ID:        uid,
		UserID:    userID,
		Token:     dbRt.Token,
		ExpiresAt: dbRt.ExpiresAt,
		Revoked:   dbRt.Revoked,
		CreatedAt: createdAt,
	}, nil
}

func (s *service) RevokeRefreshToken(token string) error {
	hashedToken := hashToken(token)
	ctx := context.Background()
	_ = s.queries.RevokeRefreshTokenByToken(ctx, hashedToken)
	_ = s.queries.RevokeRefreshTokenByToken(ctx, token)
	return nil
}

func (s *service) RevokeAllUserRefreshTokens(userID uuid.UUID) error {
	ctx := context.Background()
	return s.queries.RevokeAllUserRefreshTokens(ctx, userID.String())
}
