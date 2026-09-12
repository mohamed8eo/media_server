package database

import (
	"context"
	"database/sql"
	"time"

	"mediaserver/internal/database/sqlc"

	"github.com/google/uuid"
)

type Job struct {
	ID        uuid.UUID
	FileID    uuid.UUID
	TaskType  string
	Status    string
	Error     string
	CreatedAt time.Time
}

func (s *service) CreateJob(id, fileID uuid.UUID, taskType string) error {
	ctx := context.Background()
	return s.queries.CreateJob(ctx, sqlc.CreateJobParams{
		ID:       id.String(),
		FileID:   fileID.String(),
		TaskType: taskType,
	})
}

func (s *service) GetPendingOrProcessingJobs() ([]Job, error) {
	ctx := context.Background()
	dbJobs, err := s.queries.GetPendingOrProcessingJobs(ctx)
	if err != nil {
		return nil, err
	}

	var jobs []Job
	for _, j := range dbJobs {
		id, _ := uuid.Parse(j.ID)
		fileID, _ := uuid.Parse(j.FileID)
		jobs = append(jobs, Job{
			ID:       id,
			FileID:   fileID,
			TaskType: j.TaskType,
			Status:   j.Status,
			Error:    j.Error,
		})
	}
	return jobs, nil
}

func (s *service) UpdateJobStatus(id uuid.UUID, status string, errMsg string) error {
	ctx := context.Background()
	return s.queries.UpdateJobStatus(ctx, sqlc.UpdateJobStatusParams{
		Status: status,
		Error:  sql.NullString{String: errMsg, Valid: errMsg != ""},
		ID:     id.String(),
	})
}
