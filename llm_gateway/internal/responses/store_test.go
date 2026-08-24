package responses

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

func testStore(t *testing.T) (*Store, sqlmock.Sqlmock) {
	t.Helper()
	raw, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { raw.Close() })
	store, err := NewStore(sqlx.NewDb(raw, "sqlmock"), nil, DefaultStoreConfig())
	if err != nil {
		t.Fatal(err)
	}
	store.now = func() time.Time { return time.Unix(1700000000, 0).UTC() }
	return store, mock
}

func TestStoreGetUsesOwnerAndNonDisclosure(t *testing.T) {
	store, mock := testStore(t)
	owner := uuid.New()
	id := "resp_secret"
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, api_key_id, previous_response_id, status, stored, model,")).
		WithArgs(id, owner, store.now()).WillReturnError(sql.ErrNoRows)
	_, err := store.Get(context.Background(), owner, id)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestStoreGetItemsUsesOwnerAndStableOrdering(t *testing.T) {
	store, mock := testStore(t)
	owner := uuid.New()
	id := "resp_test"
	rows := sqlmock.NewRows([]string{"response_id", "ordinal", "direction", "item_id", "item_type", "status", "call_id", "token_count", "payload", "encrypted_payload", "created_at", "updated_at"}).
		AddRow(id, 0, "input", "item_input", "message", "", nil, 1, []byte(`{"type":"message"}`), nil, store.now(), store.now()).
		AddRow(id, 0, "output", "item_output", "message", "completed", nil, 2, []byte(`{"type":"message"}`), nil, store.now(), store.now())
	mock.ExpectQuery("FROM response_items i JOIN responses r").WithArgs(id, owner, store.now()).WillReturnRows(rows)
	items, err := store.GetItems(context.Background(), owner, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Direction != "input" || items[1].Direction != "output" {
		t.Fatalf("unexpected items: %#v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestStoreDeleteIsTenantScopedSoftDelete(t *testing.T) {
	store, mock := testStore(t)
	owner := uuid.New()
	id := "resp_test"
	mock.ExpectExec("UPDATE responses SET deleted_at=").WithArgs(store.now(), id, owner).WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.Delete(context.Background(), owner, id); err != nil {
		t.Fatal(err)
	}
	mock.ExpectExec("UPDATE responses SET deleted_at=").WithArgs(store.now(), id, owner).WillReturnResult(sqlmock.NewResult(0, 0))
	if err := store.Delete(context.Background(), owner, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("repeat delete got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestStoreCompleteRollsBackItemsWhenTerminalUpdateFails(t *testing.T) {
	store, mock := testStore(t)
	owner := uuid.New()
	id := "resp_test"
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status FROM responses").WithArgs(id, owner, store.now()).WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(StatusInProgress))
	mock.ExpectExec("INSERT INTO response_items").WithArgs(id, 0, "output", "item_test", "message", "completed", nil, 0, json.RawMessage(`{"type":"message"}`), nil).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE responses SET status=").WillReturnError(errors.New("write failed"))
	mock.ExpectRollback()
	err := store.Complete(context.Background(), owner, id, TerminalUpdate{Status: StatusCompleted, Items: []Item{{Ordinal: 0, Direction: "output", ItemID: "item_test", ItemType: "message", Status: "completed", Payload: json.RawMessage(`{"type":"message"}`)}}})
	if err == nil {
		t.Fatal("expected error")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestStoreCompleteIsIdempotentForSameTerminalStatus(t *testing.T) {
	store, mock := testStore(t)
	owner := uuid.New()
	id := "resp_test"
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status FROM responses").WithArgs(id, owner, store.now()).WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(StatusCompleted))
	mock.ExpectCommit()
	if err := store.Complete(context.Background(), owner, id, TerminalUpdate{Status: StatusCompleted}); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
