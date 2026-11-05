package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
)

const (
	defaultSqliteConfigTable   = "config_store"
	defaultSqliteAuthTable     = "auth_store"
	defaultSqliteCooldownTable = "cooldown_store"
	defaultSqliteConfigKey     = "config"
)

// SqliteStoreConfig captures configuration required to initialize a SQLite-backed store.
type SqliteStoreConfig struct {
	DBPath        string
	ConfigTable   string
	AuthTable     string
	CooldownTable string
	SpoolDir      string
}

// SqliteStore persists configuration, auth files, and cooldown state into a single SQLite database
// while mirroring files to a local spool workspace so existing file-based watchers continue operating.
type SqliteStore struct {
	db            *sql.DB
	cfg           SqliteStoreConfig
	spoolRoot     string
	configPath    string
	authDir       string
	cooldownStore *sqliteCooldownStateStore
	mu            sync.Mutex
}

// NewSqliteStore opens or creates a SQLite database and prepares the local workspace.
func NewSqliteStore(ctx context.Context, cfg SqliteStoreConfig) (*SqliteStore, error) {
	dbPath := strings.TrimSpace(cfg.DBPath)
	if dbPath == "" {
		dbPath = "data.sqlite"
	}
	cfg.DBPath = dbPath
	if cfg.ConfigTable == "" {
		cfg.ConfigTable = defaultSqliteConfigTable
	}
	if cfg.AuthTable == "" {
		cfg.AuthTable = defaultSqliteAuthTable
	}
	if cfg.CooldownTable == "" {
		cfg.CooldownTable = defaultSqliteCooldownTable
	}

	spoolRoot := strings.TrimSpace(cfg.SpoolDir)
	if spoolRoot == "" {
		if cwd, err := os.Getwd(); err == nil {
			spoolRoot = filepath.Join(cwd, "sqlitestore")
		} else {
			spoolRoot = filepath.Join(os.TempDir(), "sqlitestore")
		}
	}
	absSpool, err := filepath.Abs(spoolRoot)
	if err != nil {
		return nil, fmt.Errorf("sqlite store: resolve spool directory: %w", err)
	}

	configDir := filepath.Join(absSpool, "config")
	authDir := filepath.Join(absSpool, "auths")
	if err = os.MkdirAll(configDir, 0o700); err != nil {
		return nil, fmt.Errorf("sqlite store: create config directory: %w", err)
	}
	if err = os.MkdirAll(authDir, 0o700); err != nil {
		return nil, fmt.Errorf("sqlite store: create auth directory: %w", err)
	}

	dbDir := filepath.Dir(cfg.DBPath)
	if err = os.MkdirAll(dbDir, 0o755); err != nil {
		return nil, fmt.Errorf("sqlite store: create db directory %s: %w", dbDir, err)
	}
	_ = os.Chmod(dbDir, 0o700)

	// SQLite connection string with WAL and busy timeout
	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)", cfg.DBPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite store: open database: %w", err)
	}

	// Persistent database-level pragma: WAL mode
	if _, err = db.ExecContext(ctx, "PRAGMA journal_mode = WAL;"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite store: set WAL mode: %w", err)
	}

	// Restrict db file permission to owner
	if err = os.Chmod(cfg.DBPath, 0o600); err != nil && !errors.Is(err, os.ErrNotExist) {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite store: chmod db file: %w", err)
	}

	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(time.Hour)

	store := &SqliteStore{
		db:         db,
		cfg:        cfg,
		spoolRoot:  absSpool,
		configPath: filepath.Join(configDir, "config.yaml"),
		authDir:    authDir,
	}
	store.cooldownStore = &sqliteCooldownStateStore{store: store}
	return store, nil
}

// DB exposes the underlying *sql.DB connection pool.
func (s *SqliteStore) DB() *sql.DB {
	if s == nil {
		return nil
	}
	return s.db
}

// Close releases the SQLite connection.
func (s *SqliteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// EnsureSchema creates the configuration, auth, and cooldown tables if they don't exist.
func (s *SqliteStore) EnsureSchema(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("sqlite store: not initialized")
	}

	queries := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			id TEXT PRIMARY KEY,
			content TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`, s.cfg.ConfigTable),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			id TEXT PRIMARY KEY,
			content TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`, s.cfg.AuthTable),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			auth_id TEXT NOT NULL,
			model TEXT NOT NULL DEFAULT '',
			content TEXT NOT NULL,
			deleted INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (auth_id, model)
		);`, s.cfg.CooldownTable),
	}

	for _, q := range queries {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("sqlite store: schema exec: %w", err)
		}
	}

	return nil
}

// Bootstrap ensures the schema exists and synchronizes config/auth between SQLite and local spool.
func (s *SqliteStore) Bootstrap(ctx context.Context, exampleConfigPath string) error {
	return s.BootstrapWithSeeds(ctx, exampleConfigPath, "")
}

// BootstrapWithSeeds ensures the schema exists and seeds empty tables from existing files if provided.
func (s *SqliteStore) BootstrapWithSeeds(ctx context.Context, seedConfigPath string, seedAuthDir string) error {
	if err := s.EnsureSchema(ctx); err != nil {
		return err
	}
	if err := s.syncConfigFromDatabase(ctx, seedConfigPath); err != nil {
		return err
	}
	if err := s.seedAuthFromDirectoryIfEmpty(ctx, seedAuthDir); err != nil {
		return err
	}
	if err := s.syncAuthFromDatabase(ctx); err != nil {
		return err
	}
	return nil
}

func (s *SqliteStore) seedAuthFromDirectoryIfEmpty(ctx context.Context, seedAuthDir string) error {
	seedAuthDir = strings.TrimSpace(seedAuthDir)
	if seedAuthDir == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	var count int
	queryCount := fmt.Sprintf("SELECT COUNT(1) FROM %s", s.cfg.AuthTable)
	if err := s.db.QueryRowContext(ctx, queryCount).Scan(&count); err != nil {
		return fmt.Errorf("sqlite store: count auth records: %w", err)
	}
	if count > 0 {
		return nil // already populated
	}

	entries, err := os.ReadDir(seedAuthDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("sqlite store: read seed auth dir: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	upsert := fmt.Sprintf(`INSERT INTO %s (id, content, created_at, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET content = excluded.content, updated_at = excluded.updated_at`,
		s.cfg.AuthTable)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		filePath := filepath.Join(seedAuthDir, entry.Name())
		data, errRead := os.ReadFile(filePath)
		if errRead != nil || len(data) == 0 {
			continue
		}
		var dummy any
		if errJSON := json.Unmarshal(data, &dummy); errJSON != nil {
			continue
		}
		relID := entry.Name()
		if _, errExec := s.db.ExecContext(ctx, upsert, relID, string(data), now, now); errExec != nil {
			return fmt.Errorf("sqlite store: seed auth %s: %w", relID, errExec)
		}
	}

	return nil
}

func (s *SqliteStore) syncConfigFromDatabase(ctx context.Context, exampleConfigPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := fmt.Sprintf("SELECT content FROM %s WHERE id = ? LIMIT 1", s.cfg.ConfigTable)
	var content string
	err := s.db.QueryRowContext(ctx, query, defaultSqliteConfigKey).Scan(&content)
	now := time.Now().UTC().Format(time.RFC3339)

	if errors.Is(err, sql.ErrNoRows) {
		var seed []byte
		if strings.TrimSpace(exampleConfigPath) != "" {
			if data, errRead := os.ReadFile(exampleConfigPath); errRead == nil && len(data) > 0 {
				seed = data
			}
		}
		if len(seed) == 0 {
			seed = []byte("# Auto-generated default config\nport: 20128\n")
		}
		insertQuery := fmt.Sprintf("INSERT INTO %s (id, content, created_at, updated_at) VALUES (?, ?, ?, ?)", s.cfg.ConfigTable)
		if _, errInsert := s.db.ExecContext(ctx, insertQuery, defaultSqliteConfigKey, string(seed), now, now); errInsert != nil {
			return fmt.Errorf("sqlite store: insert default config: %w", errInsert)
		}
		content = string(seed)
	} else if err != nil {
		return fmt.Errorf("sqlite store: query config: %w", err)
	}

	return os.WriteFile(s.configPath, []byte(content), 0o600)
}

func (s *SqliteStore) syncAuthFromDatabase(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := fmt.Sprintf("SELECT id, content FROM %s", s.cfg.AuthTable)
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("sqlite store: query auth records: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id, content string
		if errScan := rows.Scan(&id, &content); errScan != nil {
			return fmt.Errorf("sqlite store: scan auth row: %w", errScan)
		}
		filePath := filepath.Join(s.authDir, filepath.FromSlash(id))
		if errMkdir := os.MkdirAll(filepath.Dir(filePath), 0o700); errMkdir != nil {
			return fmt.Errorf("sqlite store: create auth parent dir: %w", errMkdir)
		}
		if errWrite := os.WriteFile(filePath, []byte(content), 0o600); errWrite != nil {
			return fmt.Errorf("sqlite store: write auth file %s: %w", filePath, errWrite)
		}
	}

	return rows.Err()
}

// ConfigPath returns the path to the mirrored config.yaml inside the spool directory.
func (s *SqliteStore) ConfigPath() string {
	if s == nil {
		return ""
	}
	return s.configPath
}

// AuthDir returns the local directory containing mirrored auth files.
func (s *SqliteStore) AuthDir() string {
	if s == nil {
		return ""
	}
	return s.authDir
}

// WorkDir returns the root spool directory.
func (s *SqliteStore) WorkDir() string {
	if s == nil {
		return ""
	}
	return s.spoolRoot
}

// SetBaseDir is a no-op matching the authenticator interface contract.
func (s *SqliteStore) SetBaseDir(string) {}

// Save persists an Auth record into both the local workspace and the SQLite database.
func (s *SqliteStore) Save(ctx context.Context, auth *cliproxyauth.Auth) (string, error) {
	if auth == nil {
		return "", fmt.Errorf("sqlite store: auth is nil")
	}
	cliproxyauth.NormalizeCredentialMetadata(auth.Metadata)
	if errWeight := cliproxyauth.ValidateAuthWeight(auth); errWeight != nil {
		return "", fmt.Errorf("sqlite store: %w", errWeight)
	}

	path, err := s.resolveAuthPath(auth)
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", fmt.Errorf("sqlite store: missing file path attribute for %s", auth.ID)
	}

	if auth.Disabled && !cliproxyauth.HasAuthCreationIntent(ctx) {
		if _, statErr := os.Stat(path); errors.Is(statErr, fs.ErrNotExist) {
			return "", nil
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err = os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("sqlite store: create auth directory: %w", err)
	}

	var raw []byte
	switch {
	case auth.Storage != nil:
		if auth.Metadata == nil {
			auth.Metadata = make(map[string]any)
		}
		auth.Metadata["disabled"] = auth.Disabled
		if setter, ok := auth.Storage.(interface{ SetMetadata(map[string]any) }); ok {
			setter.SetMetadata(auth.Metadata)
		}
		if err = auth.Storage.SaveTokenToFile(path); err != nil {
			return "", err
		}
		raw, _ = os.ReadFile(path)
	case auth.Metadata != nil:
		auth.Metadata["disabled"] = auth.Disabled
		var errMarshal error
		raw, errMarshal = json.Marshal(auth.Metadata)
		if errMarshal != nil {
			return "", fmt.Errorf("sqlite store: marshal metadata: %w", errMarshal)
		}
		if errWrite := os.WriteFile(path, raw, 0o600); errWrite != nil {
			return "", fmt.Errorf("sqlite store: write auth file: %w", errWrite)
		}
	default:
		return "", fmt.Errorf("sqlite store: nothing to persist for %s", auth.ID)
	}

	if auth.Attributes == nil {
		auth.Attributes = make(map[string]string)
	}
	auth.Attributes[cliproxyauth.AttributePath] = path
	auth.Attributes[cliproxyauth.AttributeSourceBackend] = cliproxyauth.AuthSourceSqlite

	relID, err := s.relativeAuthID(path)
	if err != nil {
		return "", err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	upsert := fmt.Sprintf(`INSERT INTO %s (id, content, created_at, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET content = excluded.content, updated_at = excluded.updated_at`,
		s.cfg.AuthTable)

	if _, err = s.db.ExecContext(ctx, upsert, relID, string(raw), now, now); err != nil {
		return "", fmt.Errorf("sqlite store: upsert auth: %w", err)
	}

	return path, nil
}

// List enumerates all active auth records stored in the SQLite database.
func (s *SqliteStore) List(ctx context.Context) ([]*cliproxyauth.Auth, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("sqlite store: not initialized")
	}

	query := fmt.Sprintf("SELECT id, content FROM %s", s.cfg.AuthTable)
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("sqlite store: query auth: %w", err)
	}
	defer rows.Close()

	var records []*cliproxyauth.Auth
	for rows.Next() {
		var id, content string
		if errScan := rows.Scan(&id, &content); errScan != nil {
			return nil, fmt.Errorf("sqlite store: scan auth row: %w", errScan)
		}
		path := filepath.Join(s.authDir, filepath.FromSlash(id))
		authRecord := &cliproxyauth.Auth{
			ID:       id,
			FileName: filepath.Base(path),
			Attributes: map[string]string{
				cliproxyauth.AttributePath:          path,
				cliproxyauth.AttributeSourceBackend: cliproxyauth.AuthSourceSqlite,
			},
		}
		var meta map[string]any
		if errJSON := json.Unmarshal([]byte(content), &meta); errJSON == nil {
			authRecord.Metadata = meta
			if d, ok := meta["disabled"].(bool); ok {
				authRecord.Disabled = d
			}
		}
		records = append(records, authRecord)
	}

	return records, rows.Err()
}

// Get retrieves one auth record by ID.
func (s *SqliteStore) Get(ctx context.Context, id string) (*cliproxyauth.Auth, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("sqlite store: not initialized")
	}

	query := fmt.Sprintf("SELECT content FROM %s WHERE id = ? LIMIT 1", s.cfg.AuthTable)
	var content string
	err := s.db.QueryRowContext(ctx, query, id).Scan(&content)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite store: get auth %s: %w", id, err)
	}

	path := filepath.Join(s.authDir, filepath.FromSlash(id))
	authRecord := &cliproxyauth.Auth{
		ID:       id,
		FileName: filepath.Base(path),
		Attributes: map[string]string{
			cliproxyauth.AttributePath:          path,
			cliproxyauth.AttributeSourceBackend: cliproxyauth.AuthSourceSqlite,
		},
	}
	var meta map[string]any
	if errJSON := json.Unmarshal([]byte(content), &meta); errJSON == nil {
		authRecord.Metadata = meta
		if d, ok := meta["disabled"].(bool); ok {
			authRecord.Disabled = d
		}
	}
	return authRecord, nil
}

// Delete removes an auth record from both the local workspace and the SQLite database.
func (s *SqliteStore) Delete(ctx context.Context, id string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("sqlite store: not initialized")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	query := fmt.Sprintf("DELETE FROM %s WHERE id = ?", s.cfg.AuthTable)
	if _, err := s.db.ExecContext(ctx, query, id); err != nil {
		return fmt.Errorf("sqlite store: delete auth %s: %w", id, err)
	}

	filePath := filepath.Join(s.authDir, filepath.FromSlash(id))
	_ = os.Remove(filePath)
	return nil
}

// UpdateConfig writes configuration changes back to SQLite.
func (s *SqliteStore) UpdateConfig(ctx context.Context, content []byte) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("sqlite store: not initialized")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339)
	upsert := fmt.Sprintf(`INSERT INTO %s (id, content, created_at, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET content = excluded.content, updated_at = excluded.updated_at`,
		s.cfg.ConfigTable)

	if _, err := s.db.ExecContext(ctx, upsert, defaultSqliteConfigKey, string(content), now, now); err != nil {
		return fmt.Errorf("sqlite store: update config: %w", err)
	}

	return os.WriteFile(s.configPath, content, 0o600)
}

func (s *SqliteStore) resolveAuthPath(auth *cliproxyauth.Auth) (string, error) {
	fileName := strings.TrimSpace(auth.FileName)
	if fileName == "" {
		fileName = strings.TrimSpace(auth.ID)
	}
	if fileName == "" {
		return "", fmt.Errorf("sqlite store: empty auth identifier")
	}
	clean := filepath.Clean(filepath.FromSlash(fileName))
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		clean = filepath.Base(clean)
	}
	if !strings.HasSuffix(clean, ".json") {
		clean += ".json"
	}
	return filepath.Join(s.authDir, clean), nil
}

func (s *SqliteStore) relativeAuthID(fullPath string) (string, error) {
	rel, err := filepath.Rel(s.authDir, fullPath)
	if err != nil {
		return "", fmt.Errorf("sqlite store: relative path: %w", err)
	}
	return filepath.ToSlash(rel), nil
}

// PersistConfig mirrors the local configuration file to SQLite.
func (s *SqliteStore) PersistConfig(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.configPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("sqlite store: read config file: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	upsert := fmt.Sprintf(`INSERT INTO %s (id, content, created_at, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET content = excluded.content, updated_at = excluded.updated_at`,
		s.cfg.ConfigTable)
	_, err = s.db.ExecContext(ctx, upsert, defaultSqliteConfigKey, string(data), now, now)
	return err
}

// PersistAuthFiles stores the provided auth file changes in SQLite.
func (s *SqliteStore) PersistAuthFiles(ctx context.Context, _ string, paths ...string) error {
	if len(paths) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, p := range paths {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		relID, err := s.relativeAuthID(trimmed)
		if err != nil {
			abs := trimmed
			if !filepath.IsAbs(abs) {
				abs = filepath.Join(s.authDir, trimmed)
			}
			relID, err = s.relativeAuthID(abs)
			if err != nil {
				continue
			}
			trimmed = abs
		}
		data, err := os.ReadFile(trimmed)
		if err != nil {
			continue
		}
		now := time.Now().UTC().Format(time.RFC3339)
		upsert := fmt.Sprintf(`INSERT INTO %s (id, content, created_at, updated_at)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET content = excluded.content, updated_at = excluded.updated_at`,
			s.cfg.AuthTable)
		_, _ = s.db.ExecContext(ctx, upsert, relID, string(data), now, now)
	}
	return nil
}

// CooldownStateStore returns the SQLite-backed runtime cooldown store.
func (s *SqliteStore) CooldownStateStore() cliproxyauth.CooldownStateStore {
	if s == nil {
		return nil
	}
	return s.cooldownStore
}
