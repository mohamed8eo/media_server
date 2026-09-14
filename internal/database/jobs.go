package database

import (
	"context"
	"database/sql"
	"time"

	"mediaserver/internal/database/sqlc"

	"github.com/google/uuid"
)

type Job struct {
	ID        uuid.UUID `json:"id"`
	FileID    uuid.UUID `json:"file_id"`
	UserID    uuid.UUID `json:"user_id"`
	TaskType  string    `json:"task_type"`
	Status    string    `json:"status"`
	Progress  int       `json:"progress"`
	Error     string    `json:"error"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *service) CreateJob(id, fileID uuid.UUID, userID uuid.UUID, taskType string) error {
	ctx := context.Background()
	var sqlFileID sql.NullString
	if fileID != uuid.Nil {
		sqlFileID = sql.NullString{String: fileID.String(), Valid: true}
	}
	var sqlUserID sql.NullString
	if userID != uuid.Nil {
		sqlUserID = sql.NullString{String: userID.String(), Valid: true}
	}
	return s.queries.CreateJob(ctx, sqlc.CreateJobParams{
		ID:       id.String(),
		FileID:   sqlFileID,
		UserID:   sqlUserID,
		TaskType: taskType,
	})
}

func (s *service) GetJobByID(id uuid.UUID) (Job, error) {
	ctx := context.Background()
	j, err := s.queries.GetJobByID(ctx, id.String())
	if err != nil {
		return Job{}, err
	}
	jid, _ := uuid.Parse(j.ID)
	var fid uuid.UUID
	if j.FileID != "" {
		fid, _ = uuid.Parse(j.FileID)
	}
	var uid uuid.UUID
	if j.UserID != "" {
		uid, _ = uuid.Parse(j.UserID)
	}
	return Job{
		ID:       jid,
		FileID:   fid,
		UserID:   uid,
		TaskType: j.TaskType,
		Status:   j.Status,
		Progress: int(j.Progress),
		Error:    j.Error,
	}, nil
}

func (s *service) ListActiveJobsByUser(userID uuid.UUID) ([]Job, error) {
	ctx := context.Background()
	dbJobs, err := s.queries.ListActiveJobsByUser(ctx, sql.NullString{String: userID.String(), Valid: true})
	if err != nil {
		return nil, err
	}
	var jobs []Job
	for _, j := range dbJobs {
		id, _ := uuid.Parse(j.ID)
		var fileID uuid.UUID
		if j.FileID != "" {
			fileID, _ = uuid.Parse(j.FileID)
		}
		var uid uuid.UUID
		if j.UserID != "" {
			uid, _ = uuid.Parse(j.UserID)
		}
		jobs = append(jobs, Job{
			ID:       id,
			FileID:   fileID,
			UserID:   uid,
			TaskType: j.TaskType,
			Status:   j.Status,
			Progress: int(j.Progress),
			Error:    j.Error,
		})
	}
	return jobs, nil
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
		var fileID uuid.UUID
		if j.FileID != "" {
			fileID, _ = uuid.Parse(j.FileID)
		}
		var uid uuid.UUID
		if j.UserID != "" {
			uid, _ = uuid.Parse(j.UserID)
		}
		jobs = append(jobs, Job{
			ID:       id,
			FileID:   fileID,
			UserID:   uid,
			TaskType: j.TaskType,
			Status:   j.Status,
			Progress: int(j.Progress),
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

func (s *service) UpdateJobProgress(id uuid.UUID, progress int) error {
	ctx := context.Background()
	return s.queries.UpdateJobProgress(ctx, sqlc.UpdateJobProgressParams{
		Progress: int64(progress),
		ID:       id.String(),
	})
}
