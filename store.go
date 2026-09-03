package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	_ "modernc.org/sqlite"
	"time"
)

var ErrConfigLimit = errors.New("configuration limit reached")

type Store struct {
	db         *sql.DB
	maxConfigs int
	auditLimit int
	configTTL  time.Duration
}
type Result struct {
	Pairs      []string          `json:"pairs"`
	Exclusions map[string]string `json:"exclusions"`
	UpdatedAt  time.Time         `json:"updated_at"`
}
type Status struct {
	Config      Config            `json:"config"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   *time.Time        `json:"updated_at,omitempty"`
	AttemptedAt *time.Time        `json:"attempted_at,omitempty"`
	Fresh       bool              `json:"fresh"`
	Error       string            `json:"error,omitempty"`
	Pairs       []string          `json:"pairs"`
	Exclusions  map[string]string `json:"exclusions"`
}
type RefreshState struct {
	AttemptedAt time.Time
	Error       string
}

func OpenStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, q := range []string{"PRAGMA journal_mode=WAL", "PRAGMA busy_timeout=5000", "PRAGMA foreign_keys=ON", `CREATE TABLE IF NOT EXISTS configs(id TEXT PRIMARY KEY, config BLOB NOT NULL, created_at TEXT NOT NULL)`, `CREATE TABLE IF NOT EXISTS config_activity(config_id TEXT PRIMARY KEY REFERENCES configs(id), last_accessed_at TEXT NOT NULL)`, `CREATE TABLE IF NOT EXISTS results(config_id TEXT PRIMARY KEY REFERENCES configs(id), pairs BLOB NOT NULL, exclusions BLOB NOT NULL, updated_at TEXT NOT NULL)`, `CREATE TABLE IF NOT EXISTS snapshots(kind TEXT PRIMARY KEY, payload BLOB NOT NULL, updated_at TEXT NOT NULL)`, `CREATE TABLE IF NOT EXISTS refresh_state(config_id TEXT PRIMARY KEY REFERENCES configs(id), attempted_at TEXT NOT NULL, error TEXT NOT NULL)`, `CREATE TABLE IF NOT EXISTS service_state(key TEXT PRIMARY KEY, attempted_at TEXT NOT NULL, error TEXT NOT NULL)`, `CREATE TABLE IF NOT EXISTS audit(id INTEGER PRIMARY KEY, config_id TEXT, event TEXT NOT NULL, detail TEXT NOT NULL, created_at TEXT NOT NULL)`, `INSERT OR IGNORE INTO config_activity(config_id,last_accessed_at) SELECT id,created_at FROM configs`} {
		if _, err = db.Exec(q); err != nil {
			db.Close()
			return nil, fmt.Errorf("initialize database: %w", err)
		}
	}
	return &Store{db: db, maxConfigs: 1000, auditLimit: 10000, configTTL: 90 * 24 * time.Hour}, nil
}
func (s *Store) Close() error { return s.db.Close() }
func (s *Store) SetLimits(maxConfigs, auditLimit int) {
	if maxConfigs > 0 {
		s.maxConfigs = maxConfigs
	}
	if auditLimit > 0 {
		s.auditLimit = auditLimit
	}
}
func (s *Store) SetConfigTTL(ttl time.Duration) {
	if ttl > 0 {
		s.configTTL = ttl
	}
}
func (s *Store) pruneInactiveTx(tx *sql.Tx, before time.Time) error {
	rows, err := tx.Query(`SELECT config_id FROM config_activity WHERE last_accessed_at < ?`, before.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, id := range ids {
		for _, query := range []string{
			`DELETE FROM refresh_state WHERE config_id=?`,
			`DELETE FROM results WHERE config_id=?`,
			`DELETE FROM config_activity WHERE config_id=?`,
			`DELETE FROM configs WHERE id=?`,
		} {
			if _, err := tx.Exec(query, id); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) PutConfig(raw Config) (string, error) {
	c, err := raw.Canonical()
	if err != nil {
		return "", err
	}
	id := c.ID()
	b, _ := json.Marshal(c)
	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	stamp := now.Format(time.RFC3339Nano)
	var exists int
	if err := tx.QueryRow(`SELECT count(*) FROM configs WHERE id=?`, id).Scan(&exists); err != nil {
		return "", err
	}
	if exists > 0 {
		if _, err := tx.Exec(`INSERT INTO config_activity(config_id,last_accessed_at) VALUES(?,?) ON CONFLICT(config_id) DO UPDATE SET last_accessed_at=excluded.last_accessed_at`, id, stamp); err != nil {
			return "", err
		}
		return id, tx.Commit()
	}
	if err := s.pruneInactiveTx(tx, now.Add(-s.configTTL)); err != nil {
		return "", err
	}
	var count int
	if err := tx.QueryRow(`SELECT count(*) FROM configs`).Scan(&count); err != nil {
		return "", err
	}
	if count >= s.maxConfigs {
		return "", ErrConfigLimit
	}
	if _, err := tx.Exec(`INSERT INTO configs(id,config,created_at) VALUES(?,?,?)`, id, b, stamp); err != nil {
		return "", err
	}
	if _, err := tx.Exec(`INSERT INTO config_activity(config_id,last_accessed_at) VALUES(?,?)`, id, stamp); err != nil {
		return "", err
	}
	return id, tx.Commit()
}
func (s *Store) ConfigCount() int {
	var n int
	_ = s.db.QueryRow(`SELECT count(*) FROM configs`).Scan(&n)
	return n
}
func (s *Store) GetConfig(id string) (Config, time.Time, error) {
	var b []byte
	var ts string
	if err := s.db.QueryRow(`SELECT config,created_at FROM configs WHERE id=?`, id).Scan(&b, &ts); err != nil {
		return Config{}, time.Time{}, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return c, time.Time{}, err
	}
	t, err := time.Parse(time.RFC3339Nano, ts)
	return c, t, err
}
func (s *Store) TouchConfig(id string) error {
	_, err := s.db.Exec(`INSERT INTO config_activity(config_id,last_accessed_at) VALUES(?,?) ON CONFLICT(config_id) DO UPDATE SET last_accessed_at=excluded.last_accessed_at`, id, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}
func (s *Store) ListConfigs() ([]struct {
	ID     string
	Config Config
}, error) {
	rows, err := s.db.Query(`SELECT id,config FROM configs ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []struct {
		ID     string
		Config Config
	}
	for rows.Next() {
		var x struct {
			ID     string
			Config Config
		}
		var b []byte
		if err := rows.Scan(&x.ID, &b); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(b, &x.Config); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *Store) SaveResult(id string, pairs []string, ex map[string]string, at time.Time) error {
	if len(pairs) == 0 {
		return errors.New("refusing to publish empty pairlist")
	}
	if ex == nil {
		ex = map[string]string{}
	}
	pb, _ := json.Marshal(pairs)
	eb, _ := json.Marshal(ex)
	_, err := s.db.Exec(`INSERT INTO results(config_id,pairs,exclusions,updated_at) VALUES(?,?,?,?) ON CONFLICT(config_id) DO UPDATE SET pairs=excluded.pairs,exclusions=excluded.exclusions,updated_at=excluded.updated_at`, id, pb, eb, at.UTC().Format(time.RFC3339Nano))
	return err
}
func (s *Store) PublishResult(id string, pairs []string, ex map[string]string, snapshotAt, attemptedAt time.Time) error {
	if len(pairs) == 0 {
		return errors.New("refusing to publish empty pairlist")
	}
	if ex == nil {
		ex = map[string]string{}
	}
	pb, _ := json.Marshal(pairs)
	eb, _ := json.Marshal(ex)
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO results(config_id,pairs,exclusions,updated_at) VALUES(?,?,?,?) ON CONFLICT(config_id) DO UPDATE SET pairs=excluded.pairs,exclusions=excluded.exclusions,updated_at=excluded.updated_at`, id, pb, eb, snapshotAt.UTC().Format(time.RFC3339Nano)); err != nil {
		return err
	}
	var stateAttempted, stateError string
	err = tx.QueryRow(`SELECT attempted_at,error FROM refresh_state WHERE config_id=?`, id).Scan(&stateAttempted, &stateError)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		_, err = tx.Exec(`INSERT INTO refresh_state(config_id,attempted_at,error) VALUES(?,?,?)`, id, attemptedAt.UTC().Format(time.RFC3339Nano), "")
	case err != nil:
		return err
	default:
		stateAt, parseErr := time.Parse(time.RFC3339Nano, stateAttempted)
		if parseErr != nil {
			return parseErr
		}
		if stateError == "" || !stateAt.After(snapshotAt) {
			_, err = tx.Exec(`UPDATE refresh_state SET attempted_at=?,error='' WHERE config_id=?`, attemptedAt.UTC().Format(time.RFC3339Nano), id)
		}
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) GetResult(id string) (Result, error) {
	var p, e []byte
	var ts string
	if err := s.db.QueryRow(`SELECT pairs,exclusions,updated_at FROM results WHERE config_id=?`, id).Scan(&p, &e, &ts); err != nil {
		return Result{}, err
	}
	var r Result
	if err := json.Unmarshal(p, &r.Pairs); err != nil {
		return r, err
	}
	if err := json.Unmarshal(e, &r.Exclusions); err != nil {
		return r, err
	}
	r.UpdatedAt, _ = time.Parse(time.RFC3339Nano, ts)
	return r, nil
}
func (s *Store) SaveSnapshot(kind string, payload any, at time.Time) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO snapshots(kind,payload,updated_at) VALUES(?,?,?) ON CONFLICT(kind) DO UPDATE SET payload=excluded.payload,updated_at=excluded.updated_at`, kind, b, at.UTC().Format(time.RFC3339Nano))
	return err
}
func (s *Store) SaveSharedSnapshots(assets []Asset, markets []HyperMarket, at time.Time) error {
	assetsJSON, err := json.Marshal(assets)
	if err != nil {
		return err
	}
	marketsJSON, err := json.Marshal(markets)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stamp := at.UTC().Format(time.RFC3339Nano)
	for _, snapshot := range []struct {
		kind    string
		payload []byte
	}{{"lunarcrush", assetsJSON}, {"hyperliquid", marketsJSON}} {
		if _, err := tx.Exec(`INSERT INTO snapshots(kind,payload,updated_at) VALUES(?,?,?) ON CONFLICT(kind) DO UPDATE SET payload=excluded.payload,updated_at=excluded.updated_at`, snapshot.kind, snapshot.payload, stamp); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`INSERT INTO service_state(key,attempted_at,error) VALUES('refresh',?,'') ON CONFLICT(key) DO UPDATE SET attempted_at=excluded.attempted_at,error=''`, stamp); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) LoadSnapshot(kind string, target any) (time.Time, error) {
	var payload []byte
	var updated string
	if err := s.db.QueryRow(`SELECT payload,updated_at FROM snapshots WHERE kind=?`, kind).Scan(&payload, &updated); err != nil {
		return time.Time{}, err
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339Nano, updated)
}
func (s *Store) SetServiceRefreshState(publicError string, at time.Time) error {
	_, err := s.db.Exec(`INSERT INTO service_state(key,attempted_at,error) VALUES('refresh',?,?) ON CONFLICT(key) DO UPDATE SET attempted_at=excluded.attempted_at,error=excluded.error`, at.UTC().Format(time.RFC3339Nano), publicError)
	return err
}
func (s *Store) GetServiceRefreshState() (RefreshState, error) {
	var state RefreshState
	var attempted string
	if err := s.db.QueryRow(`SELECT attempted_at,error FROM service_state WHERE key='refresh'`).Scan(&attempted, &state.Error); err != nil {
		return state, err
	}
	state.AttemptedAt, _ = time.Parse(time.RFC3339Nano, attempted)
	return state, nil
}
func (s *Store) SetRefreshState(id, publicError string, at time.Time) error {
	_, err := s.db.Exec(`INSERT INTO refresh_state(config_id,attempted_at,error) VALUES(?,?,?) ON CONFLICT(config_id) DO UPDATE SET attempted_at=excluded.attempted_at,error=excluded.error`, id, at.UTC().Format(time.RFC3339Nano), publicError)
	return err
}
func (s *Store) GetRefreshState(id string) (RefreshState, error) {
	var state RefreshState
	var attempted string
	if err := s.db.QueryRow(`SELECT attempted_at,error FROM refresh_state WHERE config_id=?`, id).Scan(&attempted, &state.Error); err != nil {
		return state, err
	}
	state.AttemptedAt, _ = time.Parse(time.RFC3339Nano, attempted)
	return state, nil
}
func (s *Store) Audit(id, event, detail string) {
	tx, err := s.db.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`INSERT INTO audit(config_id,event,detail,created_at) VALUES(?,?,?,?)`, id, event, detail, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return
	}
	if _, err = tx.Exec(`DELETE FROM audit WHERE id <= (SELECT COALESCE(MAX(id)-?,0) FROM audit)`, s.auditLimit); err != nil {
		return
	}
	_ = tx.Commit()
}
