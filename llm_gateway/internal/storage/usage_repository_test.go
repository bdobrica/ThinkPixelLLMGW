package storage

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"llm_gateway/internal/models"
)

func TestUsageRepository_CreateWithTx(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock database: %v", err)
	}
	defer sqlDB.Close()

	sqlxDB := sqlx.NewDb(sqlDB, "sqlmock")
	repo := NewUsageRepository(&DB{conn: sqlxDB})

	record := &models.UsageRecord{
		ID:              uuid.New(),
		APIKeyID:        uuid.New(),
		ModelID:         uuid.New(),
		ProviderID:      uuid.New(),
		RequestID:       uuid.New(),
		ModelName:       "gpt-4",
		Endpoint:        "/v1/chat/completions",
		InputTokens:     120,
		OutputTokens:    45,
		CachedTokens:    5,
		ReasoningTokens: 7,
		ResponseTimeMS:  220,
		StatusCode:      200,
		ErrorMessage:    "",
	}

	createdAt := time.Now().UTC()
	rows := sqlmock.NewRows([]string{"created_at"}).AddRow(createdAt)
	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO usage_records").WithArgs(
		record.ID,
		record.APIKeyID,
		record.ModelID,
		record.ProviderID,
		record.RequestID,
		record.ModelName,
		record.Endpoint,
		record.InputTokens,
		record.OutputTokens,
		record.CachedTokens,
		record.ReasoningTokens,
		record.ResponseTimeMS,
		record.StatusCode,
		record.ErrorMessage,
	).WillReturnRows(rows)
	mock.ExpectCommit()

	tx, err := sqlxDB.BeginTxx(context.Background(), nil)
	if err != nil {
		t.Fatalf("failed to begin transaction: %v", err)
	}

	if err := repo.CreateWithTx(context.Background(), tx, record); err != nil {
		t.Fatalf("CreateWithTx returned error: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("failed to commit transaction: %v", err)
	}
	if !record.CreatedAt.Equal(createdAt) {
		t.Fatalf("record.CreatedAt = %v, want %v", record.CreatedAt, createdAt)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}
