package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/GinKuReNai/scythe/internal/decision"
	_ "modernc.org/sqlite"
)

type Cache struct{ db *sql.DB }

func Open(ctx context.Context, path string) (*Cache, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create cache directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("create cache database: %w", err)
	}
	if err = file.Close(); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open cache database: %w", err)
	}
	db.SetMaxOpenConns(1)
	c := &Cache{db}
	for _, query := range []string{"PRAGMA busy_timeout=5000", "PRAGMA journal_mode=WAL", `CREATE TABLE IF NOT EXISTS decisions_v1 (cache_key TEXT PRIMARY KEY, result_json TEXT NOT NULL, response_json TEXT NOT NULL, created_at INTEGER NOT NULL)`} {
		if _, err = db.ExecContext(ctx, query); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("initialize cache database: %w", err)
		}
	}
	return c, nil
}
func (c *Cache) Get(ctx context.Context, key string) (decision.Result, bool, error) {
	var data, raw string
	err := c.db.QueryRowContext(ctx, "SELECT result_json,response_json FROM decisions_v1 WHERE cache_key=?", key).Scan(&data, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return decision.Result{}, false, nil
	}
	if err != nil {
		return decision.Result{}, false, fmt.Errorf("read decision cache: %w", err)
	}
	var result decision.Result
	if err = json.Unmarshal([]byte(data), &result); err != nil {
		return result, false, fmt.Errorf("decode decision cache: %w", err)
	}
	if err = result.Validate(); err != nil {
		return result, false, fmt.Errorf("invalid cached decision: %w", err)
	}
	result.RawResponse = json.RawMessage(raw)
	return result, true, nil
}
func (c *Cache) Put(ctx context.Context, key string, r decision.Result) error {
	if err := r.Validate(); err != nil {
		return err
	}
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	_, err = c.db.ExecContext(ctx, `INSERT INTO decisions_v1(cache_key,result_json,response_json,created_at) VALUES(?,?,?,?) ON CONFLICT(cache_key) DO UPDATE SET result_json=excluded.result_json,response_json=excluded.response_json,created_at=excluded.created_at`, key, string(data), string(r.RawResponse), time.Now().Unix())
	if err != nil {
		return fmt.Errorf("write decision cache: %w", err)
	}
	return nil
}
func (c *Cache) Clean(ctx context.Context) error {
	_, err := c.db.ExecContext(ctx, "DELETE FROM decisions_v1")
	return err
}
func (c *Cache) Close() error { return c.db.Close() }
