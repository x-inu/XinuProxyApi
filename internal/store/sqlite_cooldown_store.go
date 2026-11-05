package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
)

var _ cliproxyauth.CooldownStateStoreProvider = (*SqliteStore)(nil)
var _ cliproxyauth.CooldownStateStore = (*sqliteCooldownStateStore)(nil)

type sqliteCooldownStateStore struct {
	store *SqliteStore
	mu    sync.Mutex
}

func (s *sqliteCooldownStateStore) Load(ctx context.Context) (records []cliproxyauth.CooldownStateRecord, err error) {
	if s == nil || s.store == nil || s.store.db == nil {
		return nil, fmt.Errorf("sqlite cooldown store: not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	query := fmt.Sprintf("SELECT content FROM %s WHERE deleted = 0", s.store.cfg.CooldownTable)
	rows, errQuery := s.store.db.QueryContext(ctx, query)
	if errQuery != nil {
		return nil, fmt.Errorf("sqlite cooldown store: query state: %w", errQuery)
	}
	defer rows.Close()

	for rows.Next() {
		var content string
		if errScan := rows.Scan(&content); errScan != nil {
			return nil, fmt.Errorf("sqlite cooldown store: scan record: %w", errScan)
		}
		var record cliproxyauth.CooldownStateRecord
		if errJSON := json.Unmarshal([]byte(content), &record); errJSON == nil {
			records = append(records, record)
		}
	}

	return records, rows.Err()
}

func (s *sqliteCooldownStateStore) Save(ctx context.Context, records []cliproxyauth.CooldownStateRecord) error {
	if s == nil || s.store == nil || s.store.db == nil {
		return fmt.Errorf("sqlite cooldown store: not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite cooldown store: begin tx: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC().Format(time.RFC3339)
	upsertQuery := fmt.Sprintf(`INSERT INTO %s (auth_id, model, content, deleted, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(auth_id, model) DO UPDATE SET content = excluded.content, deleted = excluded.deleted, updated_at = excluded.updated_at`,
		s.store.cfg.CooldownTable)

	stmt, err := tx.PrepareContext(ctx, upsertQuery)
	if err != nil {
		return fmt.Errorf("sqlite cooldown store: prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, rec := range records {
		if rec.AuthID == "" {
			continue
		}
		raw, errMarshal := json.Marshal(rec)
		if errMarshal != nil {
			continue
		}
		deleted := 0
		if rec.NextRetryAfter.IsZero() && rec.Status == "" {
			deleted = 1
		}
		if _, errExec := stmt.ExecContext(ctx, rec.AuthID, rec.Model, string(raw), deleted, now, now); errExec != nil {
			return fmt.Errorf("sqlite cooldown store: exec upsert: %w", errExec)
		}
	}

	return tx.Commit()
}
