package database

import (
	"context"
	"time"

	"mediaserver/internal/database/sqlc"
	"mediaserver/internal/models"

	"github.com/google/uuid"
)

func (s *service) CreateUser(email, passwordHash string) (uuid.UUID, error) {
	id := uuid.New()
	ctx := context.Background()
	err := s.queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:           id.String(),
		Email:        email,
		PasswordHash: passwordHash,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func (s *service) GetUserByEmail(email string) (*models.User, error) {
	ctx := context.Background()
	dbUser, err := s.queries.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	uid, _ := uuid.Parse(dbUser.ID)
	var createdAt time.Time
	if dbUser.CreatedAt.Valid {
		createdAt = dbUser.CreatedAt.Time
	}

	return &models.User{
		ID:           uid,
		Email:        dbUser.Email,
		PasswordHash: dbUser.PasswordHash,
		CreatedAt:    createdAt,
	}, nil
}

func (s *service) GetUserByID(id uuid.UUID) (*models.User, error) {
	ctx := context.Background()
	dbUser, err := s.queries.GetUserByID(ctx, id.String())
	if err != nil {
		return nil, err
	}

	uid, _ := uuid.Parse(dbUser.ID)
	var createdAt time.Time
	if dbUser.CreatedAt.Valid {
		createdAt = dbUser.CreatedAt.Time
	}

	return &models.User{
		ID:           uid,
		Email:        dbUser.Email,
		PasswordHash: dbUser.PasswordHash,
		CreatedAt:    createdAt,
	}, nil
}

func (s *service) UpdatePasswordByEmail(email, passwordHash string) error {
	ctx := context.Background()
	return s.queries.UpdatePasswordByEmail(ctx, sqlc.UpdatePasswordByEmailParams{
		PasswordHash: passwordHash,
		Email:        email,
	})
}
