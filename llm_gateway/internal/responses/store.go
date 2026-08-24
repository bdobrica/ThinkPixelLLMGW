package responses

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

var (
	// ErrNotFound deliberately covers absent, foreign, expired, deleted, and
	// non-stored responses so callers cannot probe another tenant's IDs.
	ErrNotFound       = errors.New("response not found")
	ErrInvalidState   = errors.New("invalid response state transition")
	ErrTraversalLimit = errors.New("response predecessor traversal limit exceeded")
)

type Cipher interface {
	Encrypt([]byte) (string, error)
	Decrypt(string) ([]byte, error)
}

type StoreConfig struct {
	Retention          time.Duration
	TransientRetention time.Duration
	OrphanedAfter      time.Duration
	MaxChainDepth      int
	MaxChainItems      int
	MaxChainBytes      int
	MaxChainTokens     int
}

func DefaultStoreConfig() StoreConfig {
	return StoreConfig{
		Retention: 30 * 24 * time.Hour, TransientRetention: time.Hour,
		OrphanedAfter: 15 * time.Minute, MaxChainDepth: 64,
		MaxChainItems: 4096, MaxChainBytes: 16 << 20, MaxChainTokens: 128000,
	}
}

type Record struct {
	ID                    string          `db:"id"`
	APIKeyID              uuid.UUID       `db:"api_key_id"`
	PreviousResponseID    *string         `db:"previous_response_id"`
	Status                Status          `db:"status"`
	Stored                bool            `db:"stored"`
	Model                 string          `db:"model"`
	Request               json.RawMessage `db:"request"`
	Usage                 json.RawMessage `db:"usage"`
	Error                 json.RawMessage `db:"error"`
	IncompleteDetails     json.RawMessage `db:"incomplete_details"`
	ProviderCorrelationID *string         `db:"provider_correlation_id"`
	ExpiresAt             time.Time       `db:"expires_at"`
	CreatedAt             time.Time       `db:"created_at"`
	StartedAt             *time.Time      `db:"started_at"`
	CompletedAt           *time.Time      `db:"completed_at"`
	UpdatedAt             time.Time       `db:"updated_at"`
}

type Item struct {
	ResponseID       string          `db:"response_id"`
	Ordinal          int             `db:"ordinal"`
	Direction        string          `db:"direction"`
	ItemID           string          `db:"item_id"`
	ItemType         string          `db:"item_type"`
	Status           string          `db:"status"`
	CallID           *string         `db:"call_id"`
	TokenCount       int             `db:"token_count"`
	Payload          json.RawMessage `db:"payload"`
	EncryptedPayload *string         `db:"encrypted_payload"`
	CreatedAt        time.Time       `db:"created_at"`
	UpdatedAt        time.Time       `db:"updated_at"`
}

type CreateRecord struct {
	ID                    string
	APIKeyID              uuid.UUID
	PreviousResponseID    *string
	Model                 string
	Stored                bool
	Request               json.RawMessage
	ProviderCorrelationID *string
}

type TerminalUpdate struct {
	Status                          Status
	Items                           []Item
	Usage, Error, IncompleteDetails json.RawMessage
}

type ToolExecution struct {
	ID                    string          `db:"id"`
	ResponseID            string          `db:"response_id"`
	APIKeyID              uuid.UUID       `db:"api_key_id"`
	CallID                string          `db:"call_id"`
	ToolType              string          `db:"tool_type"`
	Status                Status          `db:"status"`
	ProviderCorrelationID *string         `db:"provider_correlation_id"`
	Request               json.RawMessage `db:"request"`
	Result                json.RawMessage `db:"result"`
	Error                 json.RawMessage `db:"error"`
}

type Store struct {
	db     *sqlx.DB
	cipher Cipher
	cfg    StoreConfig
	now    func() time.Time
}

func NewStore(db *sqlx.DB, cipher Cipher, cfg StoreConfig) (*Store, error) {
	if db == nil || cfg.Retention <= 0 || cfg.TransientRetention <= 0 || cfg.OrphanedAfter <= 0 ||
		cfg.MaxChainDepth <= 0 || cfg.MaxChainItems <= 0 || cfg.MaxChainBytes <= 0 || cfg.MaxChainTokens <= 0 {
		return nil, errors.New("invalid responses store configuration")
	}
	return &Store{db: db, cipher: cipher, cfg: cfg, now: time.Now}, nil
}

func (s *Store) Create(ctx context.Context, input CreateRecord, items []Item) error {
	if input.ID == "" || input.APIKeyID == uuid.Nil || input.Model == "" {
		return errors.New("response id, owner, and model are required")
	}
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin response create: %w", err)
	}
	defer tx.Rollback()
	ttl := s.cfg.Retention
	if !input.Stored {
		ttl = s.cfg.TransientRetention
	}
	now := s.now().UTC()
	result, err := tx.ExecContext(ctx, `INSERT INTO responses
		(id, api_key_id, previous_response_id, status, stored, model, request, provider_correlation_id, expires_at, created_at, updated_at)
		SELECT $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10
		WHERE $3::text IS NULL OR EXISTS (SELECT 1 FROM responses p WHERE p.id=$3 AND p.api_key_id=$2
			AND p.stored=true AND p.deleted_at IS NULL AND p.expires_at>$10 AND p.status IN ('completed','incomplete'))`, input.ID, input.APIKeyID, input.PreviousResponseID,
		StatusQueued, input.Stored, input.Model, jsonOrEmpty(input.Request), input.ProviderCorrelationID, now.Add(ttl), now)
	if err != nil {
		return fmt.Errorf("insert response: %w", err)
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return ErrNotFound
	}
	for i := range items {
		if err := insertItem(ctx, tx, input.ID, &items[i], s.cipher); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit response create: %w", err)
	}
	return nil
}

func (s *Store) Get(ctx context.Context, owner uuid.UUID, id string) (*Record, error) {
	var result Record
	err := s.db.GetContext(ctx, &result, `SELECT id, api_key_id, previous_response_id, status, stored, model,
		request, usage, error, incomplete_details, provider_correlation_id, expires_at, created_at, started_at, completed_at, updated_at
		FROM responses WHERE id=$1 AND api_key_id=$2 AND stored=true AND deleted_at IS NULL AND expires_at > $3`, id, owner, s.now().UTC())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("retrieve response: %w", err)
	}
	return &result, nil
}

func (s *Store) GetItems(ctx context.Context, owner uuid.UUID, id string) ([]Item, error) {
	var items []Item
	err := s.db.SelectContext(ctx, &items, `SELECT i.response_id, i.ordinal, i.direction, i.item_id, i.item_type, i.status,
		i.call_id, i.token_count, i.payload, i.encrypted_payload, i.created_at, i.updated_at
		FROM response_items i JOIN responses r ON r.id=i.response_id
		WHERE r.id=$1 AND r.api_key_id=$2 AND r.stored=true AND r.deleted_at IS NULL AND r.expires_at>$3
		ORDER BY CASE i.direction WHEN 'input' THEN 0 ELSE 1 END, i.ordinal`, id, owner, s.now().UTC())
	if err != nil {
		return nil, fmt.Errorf("retrieve response items: %w", err)
	}
	return items, nil
}

func (s *Store) AppendItem(ctx context.Context, owner uuid.UUID, item *Item) error {
	if item.EncryptedPayload != nil && s.cipher == nil {
		return errors.New("encrypted payload requires configured cipher")
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO response_items
		(response_id, ordinal, direction, item_id, item_type, status, call_id, token_count, payload, encrypted_payload)
		SELECT $1,$2,$3,$4,$5,$6,$7,$8,$9,$10 FROM responses
		WHERE id=$1 AND api_key_id=$11 AND deleted_at IS NULL AND expires_at>$12 AND status IN ('queued','in_progress')`,
		item.ResponseID, item.Ordinal, item.Direction, item.ItemID, item.ItemType, item.Status, item.CallID,
		item.TokenCount, jsonOrEmpty(item.Payload), item.EncryptedPayload, owner, s.now().UTC())
	if err != nil {
		return fmt.Errorf("append response item: %w", err)
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) UpdateItem(ctx context.Context, owner uuid.UUID, item *Item) error {
	if item.EncryptedPayload != nil && s.cipher == nil {
		return errors.New("encrypted payload requires configured cipher")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE response_items i SET status=$1, token_count=$2, payload=$3, encrypted_payload=$4, updated_at=$5
		FROM responses r WHERE i.response_id=r.id AND i.response_id=$6 AND i.direction=$7 AND i.ordinal=$8
		AND r.api_key_id=$9 AND r.deleted_at IS NULL AND r.status IN ('queued','in_progress')`, item.Status,
		item.TokenCount, jsonOrEmpty(item.Payload), item.EncryptedPayload, s.now().UTC(), item.ResponseID, item.Direction, item.Ordinal, owner)
	if err != nil {
		return fmt.Errorf("update response item: %w", err)
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) MarkInProgress(ctx context.Context, owner uuid.UUID, id string) error {
	return s.transition(ctx, owner, id, []Status{StatusQueued}, StatusInProgress)
}

func (s *Store) SetProviderCorrelationID(ctx context.Context, owner uuid.UUID, id, correlationID string) error {
	if correlationID == "" {
		return errors.New("provider correlation id is required")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE responses SET provider_correlation_id=$1, updated_at=$2
		WHERE id=$3 AND api_key_id=$4 AND deleted_at IS NULL AND expires_at>$2 AND status IN ('queued','in_progress')`,
		correlationID, s.now().UTC(), id, owner)
	if err != nil {
		return fmt.Errorf("set response provider correlation id: %w", err)
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return ErrInvalidState
	}
	return nil
}

func (s *Store) Complete(ctx context.Context, owner uuid.UUID, id string, update TerminalUpdate) error {
	if !isTerminal(update.Status) {
		return ErrInvalidState
	}
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var current Status
	err = tx.GetContext(ctx, &current, `SELECT status FROM responses WHERE id=$1 AND api_key_id=$2
		AND deleted_at IS NULL AND expires_at>$3 FOR UPDATE`, id, owner, s.now().UTC())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("lock response for terminalization: %w", err)
	}
	if isTerminal(current) {
		if current == update.Status {
			return tx.Commit()
		}
		return ErrInvalidState
	}
	for i := range update.Items {
		if err := insertItem(ctx, tx, id, &update.Items[i], s.cipher); err != nil {
			return err
		}
	}
	now := s.now().UTC()
	result, err := tx.ExecContext(ctx, `UPDATE responses SET status=$1, usage=$2, error=$3, incomplete_details=$4,
		completed_at=$5, updated_at=$5 WHERE id=$6 AND api_key_id=$7 AND status IN ('queued','in_progress')`,
		update.Status, nullJSON(update.Usage), nullJSON(update.Error), nullJSON(update.IncompleteDetails), now, id, owner)
	if err != nil {
		return fmt.Errorf("terminalize response: %w", err)
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return ErrInvalidState
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit response terminalization: %w", err)
	}
	return nil
}

func (s *Store) Delete(ctx context.Context, owner uuid.UUID, id string) error {
	now := s.now().UTC()
	result, err := s.db.ExecContext(ctx, `UPDATE responses SET deleted_at=$1, updated_at=$1
		WHERE id=$2 AND api_key_id=$3 AND stored=true AND deleted_at IS NULL AND expires_at>$1`, now, id, owner)
	if err != nil {
		return fmt.Errorf("delete response: %w", err)
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) LoadChain(ctx context.Context, owner uuid.UUID, id string) ([]Record, []Item, error) {
	rows, err := s.db.QueryxContext(ctx, `WITH RECURSIVE chain AS (
		SELECT r.*, 1 AS depth FROM responses r WHERE r.id=$1 AND r.api_key_id=$2 AND r.stored=true AND r.deleted_at IS NULL AND r.expires_at>$3
		UNION ALL SELECT p.*, c.depth+1 FROM responses p JOIN chain c ON p.id=c.previous_response_id
		WHERE p.api_key_id=$2 AND p.stored=true AND p.deleted_at IS NULL AND p.expires_at>$3 AND c.depth<$4)
		SELECT id, api_key_id, previous_response_id, status, stored, model, request, usage, error, incomplete_details,
		provider_correlation_id, expires_at, created_at, started_at, completed_at, updated_at FROM chain ORDER BY depth DESC`,
		id, owner, s.now().UTC(), s.cfg.MaxChainDepth+1)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var records []Record
	for rows.Next() {
		var record Record
		if err := rows.StructScan(&record); err != nil {
			return nil, nil, err
		}
		records = append(records, record)
	}
	if len(records) == 0 {
		return nil, nil, ErrNotFound
	}
	if len(records) > s.cfg.MaxChainDepth {
		return nil, nil, ErrTraversalLimit
	}
	ids := make([]string, len(records))
	for i := range records {
		ids[i] = records[i].ID
	}
	var items []Item
	err = s.db.SelectContext(ctx, &items, `SELECT response_id, ordinal, direction, item_id, item_type, status, call_id, token_count,
		payload, encrypted_payload, created_at, updated_at FROM response_items WHERE response_id=ANY($1)
		ORDER BY array_position($1::text[],response_id), CASE direction WHEN 'input' THEN 0 ELSE 1 END, ordinal`, pq.Array(ids))
	if err != nil {
		return nil, nil, err
	}
	total, tokens := 0, 0
	for i := range items {
		total += len(items[i].Payload)
		tokens += items[i].TokenCount
		if items[i].EncryptedPayload != nil {
			total += len(*items[i].EncryptedPayload)
		}
	}
	if len(items) > s.cfg.MaxChainItems || total > s.cfg.MaxChainBytes || tokens > s.cfg.MaxChainTokens {
		return nil, nil, ErrTraversalLimit
	}
	return records, items, nil
}

func (s *Store) CleanupExpired(ctx context.Context, limit int) (int64, error) {
	if limit <= 0 {
		return 0, errors.New("cleanup limit must be positive")
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM responses WHERE id IN
		(SELECT id FROM responses WHERE expires_at<=$1 ORDER BY expires_at LIMIT $2 FOR UPDATE SKIP LOCKED)`, s.now().UTC(), limit)
	if err != nil {
		return 0, fmt.Errorf("cleanup expired responses: %w", err)
	}
	return result.RowsAffected()
}

func (s *Store) ReconcileOrphans(ctx context.Context, limit int) (int64, error) {
	if limit <= 0 {
		return 0, errors.New("reconcile limit must be positive")
	}
	now := s.now().UTC()
	result, err := s.db.ExecContext(ctx, `UPDATE responses SET status='failed', error=$1, completed_at=$2, updated_at=$2 WHERE id IN
		(SELECT id FROM responses WHERE status='in_progress' AND updated_at<$3 ORDER BY updated_at LIMIT $4 FOR UPDATE SKIP LOCKED)`,
		json.RawMessage(`{"code":"server_error","message":"Response interrupted before completion"}`), now, now.Add(-s.cfg.OrphanedAfter), limit)
	if err != nil {
		return 0, fmt.Errorf("reconcile responses: %w", err)
	}
	return result.RowsAffected()
}

func (s *Store) transition(ctx context.Context, owner uuid.UUID, id string, from []Status, to Status) error {
	result, err := s.db.ExecContext(ctx, `UPDATE responses SET status=$1, started_at=CASE WHEN $1='in_progress' THEN $2 ELSE started_at END,
		updated_at=$2 WHERE id=$3 AND api_key_id=$4 AND deleted_at IS NULL AND expires_at>$2 AND status=ANY($5)`, to, s.now().UTC(), id, owner, pq.Array(from))
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return ErrInvalidState
	}
	return nil
}

func insertItem(ctx context.Context, tx *sqlx.Tx, responseID string, item *Item, cipher Cipher) error {
	if item.ResponseID != "" && item.ResponseID != responseID {
		return errors.New("item response id mismatch")
	}
	item.ResponseID = responseID
	if item.EncryptedPayload != nil && cipher == nil {
		return errors.New("encrypted payload requires configured cipher")
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO response_items
		(response_id, ordinal, direction, item_id, item_type, status, call_id, token_count, payload, encrypted_payload)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, responseID, item.Ordinal, item.Direction, item.ItemID, item.ItemType, item.Status, item.CallID, item.TokenCount, jsonOrEmpty(item.Payload), item.EncryptedPayload)
	if err != nil {
		return fmt.Errorf("insert response item: %w", err)
	}
	return nil
}

// EncryptOpaquePayload seals provider-supplied reasoning state before storage.
// Plaintext must not also be copied into the item's JSON payload.
func (s *Store) EncryptOpaquePayload(plaintext []byte) (*string, error) {
	if len(plaintext) == 0 {
		return nil, nil
	}
	if s.cipher == nil {
		return nil, errors.New("opaque response encryption is not configured")
	}
	sealed, err := s.cipher.Encrypt(plaintext)
	if err != nil {
		return nil, fmt.Errorf("encrypt opaque response payload: %w", err)
	}
	return &sealed, nil
}

func (s *Store) DecryptOpaquePayload(ciphertext string) ([]byte, error) {
	if s.cipher == nil {
		return nil, errors.New("opaque response encryption is not configured")
	}
	return s.cipher.Decrypt(ciphertext)
}

func (s *Store) CreateToolExecution(ctx context.Context, execution *ToolExecution) error {
	result, err := s.db.ExecContext(ctx, `INSERT INTO response_tool_executions
		(id,response_id,api_key_id,call_id,tool_type,status,provider_correlation_id,request)
		SELECT $1,$2,$3,$4,$5,$6,$7,$8 FROM responses WHERE id=$2 AND api_key_id=$3
		AND deleted_at IS NULL AND expires_at>$9 AND status IN ('queued','in_progress')`, execution.ID,
		execution.ResponseID, execution.APIKeyID, execution.CallID, execution.ToolType, execution.Status,
		execution.ProviderCorrelationID, jsonOrEmpty(execution.Request), s.now().UTC())
	if err != nil {
		return fmt.Errorf("create tool execution: %w", err)
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CompleteToolExecution(ctx context.Context, owner uuid.UUID, id string, status Status, result, failure json.RawMessage) error {
	if status != StatusCompleted && status != StatusFailed && status != StatusCancelled {
		return ErrInvalidState
	}
	now := s.now().UTC()
	execResult, err := s.db.ExecContext(ctx, `UPDATE response_tool_executions SET status=$1,result=$2,error=$3,
		completed_at=$4,updated_at=$4 WHERE id=$5 AND api_key_id=$6 AND status IN ('queued','in_progress')`,
		status, nullJSON(result), nullJSON(failure), now, id, owner)
	if err != nil {
		return fmt.Errorf("complete tool execution: %w", err)
	}
	count, _ := execResult.RowsAffected()
	if count != 1 {
		return ErrInvalidState
	}
	return nil
}

func isTerminal(status Status) bool {
	return status == StatusCompleted || status == StatusIncomplete || status == StatusFailed || status == StatusCancelled
}
func jsonOrEmpty(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return json.RawMessage(`{}`)
	}
	return value
}
func nullJSON(value json.RawMessage) any {
	if len(value) == 0 {
		return nil
	}
	return value
}
