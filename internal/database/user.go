package database

import (
	"mediaserver/internal/models"

	"github.com/google/uuid"
)

func (s *service) CreateUser(email, passwordHash string) (uuid.UUID, error) {
	id := uuid.New()
	_, err := s.db.Exec(
		`INSERT INTO users (id, email, password_hash) VALUES (?, ?, ?)`,
		id.String(), email, passwordHash,
	)
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func (s *service) GetUserByEmail(email string) (*models.User, error) {
	row := s.db.QueryRow(
		`SELECT id, email, password_hash, created_at FROM users WHERE email = ?`,
		email,
	)

	var u models.User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
