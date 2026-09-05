package saga

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// SQLDialect identifies the database dialect for SQL generation.
type SQLDialect int

const (
	// DialectPostgreSQL is the PostgreSQL dialect.
	DialectPostgreSQL SQLDialect = iota
	// DialectSQLite is the SQLite dialect.
	DialectSQLite
)

// SQLStore is a database/sql implementation of both SagaStore and
// OutboxStore. It supports PostgreSQL and SQLite via dialect-aware SQL.
type SQLStore struct {
	db      *sql.DB
	dialect SQLDialect
	clock   func() time.Time
}

// NewSQLStore creates a new SQLStore for the given dialect. Call Init
// once before use to create the required tables.
func NewSQLStore(db *sql.DB, dialect SQLDialect) (*SQLStore, error) {
	if db == nil {
		return nil, fmt.Errorf("saga: nil database")
	}
	return &SQLStore{
		db:      db,
		dialect: dialect,
		clock:   time.Now,
	}, nil
}

// Init creates the saga_state and saga_outbox tables if they do not
// already exist. Must be called once before use.
func (s *SQLStore) Init(ctx context.Context) error {
	if err := s.createSagaTable(ctx); err != nil {
		return fmt.Errorf("saga: create saga_state table: %w", err)
	}
	if err := s.createOutboxTable(ctx); err != nil {
		return fmt.Errorf("saga: create saga_outbox table: %w", err)
	}
	return nil
}

func (s *SQLStore) createSagaTable(ctx context.Context) error {
	var stmt string
	switch s.dialect {
	case DialectPostgreSQL:
		stmt = `CREATE TABLE IF NOT EXISTS saga_state (
			id          TEXT PRIMARY KEY,
			saga_name   TEXT NOT NULL,
			status      TEXT NOT NULL,
			steps       JSONB NOT NULL,
			data        JSONB,
			started_at  TIMESTAMPTZ NOT NULL,
			updated_at  TIMESTAMPTZ NOT NULL,
			claimed_by  TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_saga_status ON saga_state(status) WHERE status IN ('Running', 'Compensating');`
	case DialectSQLite:
		stmt = `CREATE TABLE IF NOT EXISTS saga_state (
			id          TEXT PRIMARY KEY,
			saga_name   TEXT NOT NULL,
			status      TEXT NOT NULL,
			steps       TEXT NOT NULL,
			data        TEXT,
			started_at  TEXT NOT NULL,
			updated_at  TEXT NOT NULL,
			claimed_by  TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_saga_status ON saga_state(status);`
	}
	_, err := s.db.ExecContext(ctx, stmt)
	return err
}

func (s *SQLStore) createOutboxTable(ctx context.Context) error {
	var stmt string
	switch s.dialect {
	case DialectPostgreSQL:
		stmt = `CREATE TABLE IF NOT EXISTS saga_outbox (
			id           TEXT PRIMARY KEY,
			saga_id      TEXT NOT NULL,
			step_name    TEXT NOT NULL,
			event_type   TEXT NOT NULL,
			payload      BYTEA NOT NULL,
			created_at   TIMESTAMPTZ NOT NULL,
			published_at TIMESTAMPTZ,
			status       TEXT NOT NULL DEFAULT 'Pending',
			fail_count   INTEGER NOT NULL DEFAULT 0,
			last_error   TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_outbox_status ON saga_outbox(status) WHERE status = 'Pending';`
	case DialectSQLite:
		stmt = `CREATE TABLE IF NOT EXISTS saga_outbox (
			id           TEXT PRIMARY KEY,
			saga_id      TEXT NOT NULL,
			step_name    TEXT NOT NULL,
			event_type   TEXT NOT NULL,
			payload      BLOB NOT NULL,
			created_at   TEXT NOT NULL,
			published_at TEXT,
			status       TEXT NOT NULL DEFAULT 'Pending',
			fail_count   INTEGER NOT NULL DEFAULT 0,
			last_error   TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_outbox_status ON saga_outbox(status);`
	}
	_, err := s.db.ExecContext(ctx, stmt)
	return err
}

// Save creates or replaces a saga state record.
func (s *SQLStore) Save(ctx context.Context, st *SagaState) error {
	stepsJSON, err := json.Marshal(st.Steps)
	if err != nil {
		return fmt.Errorf("saga: marshal steps: %w", err)
	}
	var dataJSON []byte
	if st.Data != nil {
		dataJSON, err = json.Marshal(st.Data)
		if err != nil {
			return fmt.Errorf("saga: marshal data: %w", err)
		}
	}
	now := s.clock()
	st.UpdatedAt = now

	_, err = s.db.ExecContext(ctx, `INSERT INTO saga_state (id, saga_name, status, steps, data, started_at, updated_at, claimed_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULL)
		ON CONFLICT(id) DO UPDATE SET
			saga_name = excluded.saga_name,
			status = excluded.status,
			steps = excluded.steps,
			data = excluded.data,
			updated_at = excluded.updated_at`,
		st.ID, st.SagaName, string(st.Status), stepsJSON, dataJSON, st.StartedAt, now)
	return err
}

// Load retrieves a saga state by ID.
func (s *SQLStore) Load(ctx context.Context, sagaID string) (*SagaState, error) {
	var (
		st        SagaState
		statusStr string
		stepsJSON []byte
		dataJSON  []byte
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT id, saga_name, status, steps, data, started_at, updated_at FROM saga_state WHERE id = $1`,
		sagaID,
	).Scan(&st.ID, &st.SagaName, &statusStr, &stepsJSON, &dataJSON, &st.StartedAt, &st.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("saga: not found: %s", sagaID)
		}
		return nil, err
	}
	st.Status = SagaStatus(statusStr)
	if err := json.Unmarshal(stepsJSON, &st.Steps); err != nil {
		return nil, fmt.Errorf("saga: unmarshal steps: %w", err)
	}
	if dataJSON != nil {
		if err := json.Unmarshal(dataJSON, &st.Data); err != nil {
			return nil, fmt.Errorf("saga: unmarshal data: %w", err)
		}
	}
	return &st, nil
}

// UpdateStepAndOutbox atomically updates a step's state and appends
// outbox records in a single transaction.
func (s *SQLStore) UpdateStepAndOutbox(ctx context.Context, sagaID string, stepIdx int, step StepState, outbox []OutboxRecord) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("saga: begin tx: %w", err)
	}
	defer tx.Rollback()

	st, err := s.loadForUpdate(ctx, tx, sagaID)
	if err != nil {
		return err
	}

	if stepIdx < 0 || stepIdx >= len(st.Steps) {
		return fmt.Errorf("saga: step index out of range: %d", stepIdx)
	}

	st.Steps[stepIdx] = step
	st.UpdatedAt = s.clock()

	stepsJSON, err := json.Marshal(st.Steps)
	if err != nil {
		return fmt.Errorf("saga: marshal steps: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE saga_state SET steps = $1, updated_at = $2 WHERE id = $3`,
		stepsJSON, st.UpdatedAt, sagaID)
	if err != nil {
		return fmt.Errorf("saga: update step: %w", err)
	}

	for _, rec := range outbox {
		if rec.ID == "" {
			return fmt.Errorf("saga: outbox record ID cannot be empty")
		}
		status := rec.Status
		if status == "" {
			status = OutboxPending
		}
		_, err = tx.ExecContext(ctx,
			`INSERT INTO saga_outbox (id, saga_id, step_name, event_type, payload, created_at, status, fail_count, last_error)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			rec.ID, rec.SagaID, rec.StepName, rec.EventType, rec.Payload, rec.CreatedAt, string(status), rec.FailCount, rec.LastError)
		if err != nil {
			return fmt.Errorf("saga: insert outbox record: %w", err)
		}
	}

	return tx.Commit()
}

func (s *SQLStore) loadForUpdate(ctx context.Context, tx *sql.Tx, sagaID string) (*SagaState, error) {
	var (
		st        SagaState
		statusStr string
		stepsJSON []byte
		dataJSON  []byte
	)
	query := `SELECT id, saga_name, status, steps, data, started_at, updated_at FROM saga_state WHERE id = $1`
	if s.dialect == DialectPostgreSQL {
		query += " FOR UPDATE"
	}
	err := tx.QueryRowContext(ctx, query, sagaID).Scan(
		&st.ID, &st.SagaName, &statusStr, &stepsJSON, &dataJSON, &st.StartedAt, &st.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("saga: not found: %s", sagaID)
		}
		return nil, err
	}
	st.Status = SagaStatus(statusStr)
	if err := json.Unmarshal(stepsJSON, &st.Steps); err != nil {
		return nil, fmt.Errorf("saga: unmarshal steps: %w", err)
	}
	if dataJSON != nil {
		if err := json.Unmarshal(dataJSON, &st.Data); err != nil {
			return nil, fmt.Errorf("saga: unmarshal data: %w", err)
		}
	}
	return &st, nil
}

// ListIncomplete returns all sagas with status Running or Compensating.
func (s *SQLStore) ListIncomplete(ctx context.Context) ([]*SagaState, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, saga_name, status, steps, data, started_at, updated_at
		FROM saga_state WHERE status IN ('Running', 'Compensating')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*SagaState
	for rows.Next() {
		var (
			st        SagaState
			statusStr string
			stepsJSON []byte
			dataJSON  []byte
		)
		if err := rows.Scan(&st.ID, &st.SagaName, &statusStr, &stepsJSON, &dataJSON, &st.StartedAt, &st.UpdatedAt); err != nil {
			return nil, err
		}
		st.Status = SagaStatus(statusStr)
		if err := json.Unmarshal(stepsJSON, &st.Steps); err != nil {
			return nil, fmt.Errorf("saga: unmarshal steps: %w", err)
		}
		if dataJSON != nil {
			if err := json.Unmarshal(dataJSON, &st.Data); err != nil {
				return nil, fmt.Errorf("saga: unmarshal data: %w", err)
			}
		}
		result = append(result, &st)
	}
	return result, rows.Err()
}

// ClaimSaga atomically marks a saga as being resumed by this instance.
func (s *SQLStore) ClaimSaga(ctx context.Context, sagaID string, claimedBy string) (bool, error) {
	var query string
	switch s.dialect {
	case DialectPostgreSQL:
		query = `UPDATE saga_state SET claimed_by = $1 WHERE id = $2 AND (claimed_by IS NULL OR claimed_by = $1)`
	case DialectSQLite:
		query = `UPDATE saga_state SET claimed_by = $1 WHERE id = $2 AND (claimed_by IS NULL OR claimed_by = $1)`
	}
	result, err := s.db.ExecContext(ctx, query, claimedBy, sagaID)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// ClearClaim releases the claim on a saga.
func (s *SQLStore) ClearClaim(ctx context.Context, sagaID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE saga_state SET claimed_by = NULL WHERE id = $1`, sagaID)
	return err
}

// AppendOutbox adds a record to the outbox table.
func (s *SQLStore) AppendOutbox(ctx context.Context, rec OutboxRecord) error {
	if rec.ID == "" {
		return fmt.Errorf("saga: outbox record ID cannot be empty")
	}
	status := rec.Status
	if status == "" {
		status = OutboxPending
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO saga_outbox (id, saga_id, step_name, event_type, payload, created_at, status, fail_count, last_error)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		rec.ID, rec.SagaID, rec.StepName, rec.EventType, rec.Payload, rec.CreatedAt, string(status), rec.FailCount, rec.LastError)
	return err
}

// ListPendingOutbox returns up to limit pending outbox records.
func (s *SQLStore) ListPendingOutbox(ctx context.Context, limit int) ([]OutboxRecord, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, saga_id, step_name, event_type, payload, created_at, published_at, status, fail_count, last_error
		FROM saga_outbox WHERE status = 'Pending' ORDER BY created_at LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []OutboxRecord
	for rows.Next() {
		var (
			rec         OutboxRecord
			statusStr   string
			publishedAt sql.NullTime
			lastError   sql.NullString
		)
		if err := rows.Scan(&rec.ID, &rec.SagaID, &rec.StepName, &rec.EventType, &rec.Payload,
			&rec.CreatedAt, &publishedAt, &statusStr, &rec.FailCount, &lastError); err != nil {
			return nil, err
		}
		rec.Status = OutboxStatus(statusStr)
		if publishedAt.Valid {
			rec.PublishedAt = &publishedAt.Time
		}
		if lastError.Valid {
			rec.LastError = lastError.String
		}
		result = append(result, rec)
	}
	return result, rows.Err()
}

// MarkPublished marks a record as successfully published.
func (s *SQLStore) MarkPublished(ctx context.Context, id string) error {
	now := s.clock()
	_, err := s.db.ExecContext(ctx,
		`UPDATE saga_outbox SET status = 'Published', published_at = $1, last_error = '' WHERE id = $2`,
		now, id)
	return err
}

// MarkFailed increments the fail count and, if the threshold is
// reached, marks the record as Failed.
func (s *SQLStore) MarkFailed(ctx context.Context, id string, errMsg string, threshold int) error {
	if threshold > 0 {
		_, err := s.db.ExecContext(ctx,
			`UPDATE saga_outbox
			SET fail_count = fail_count + 1,
			    last_error = $1,
			    status = CASE WHEN fail_count + 1 >= $2 THEN 'Failed' ELSE status END
			WHERE id = $3`,
			errMsg, threshold, id)
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE saga_outbox SET fail_count = fail_count + 1, last_error = $1 WHERE id = $2`,
		errMsg, id)
	return err
}
