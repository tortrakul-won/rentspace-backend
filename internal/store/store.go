package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Store extends Querier with transactional execution and batch config reads.
type Store interface {
	Querier
	ExecTx(ctx context.Context, fn func(Querier) error) error
	// GetSystemConfigMultiple fetches multiple system_config keys in one round-trip.
	// Missing keys are absent from the returned map (no error).
	GetSystemConfigMultiple(ctx context.Context, keys []string) (map[string]string, error)
}

// SQLStore is the production implementation backed by *sql.DB.
type SQLStore struct {
	*Queries
	db *sql.DB
}

func NewStore(db *sql.DB) *SQLStore {
	return &SQLStore{Queries: New(db), db: db}
}

// ExecTx runs fn inside a transaction. If fn returns an error the transaction
// is rolled back; otherwise it is committed.
func (s *SQLStore) ExecTx(ctx context.Context, fn func(Querier) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	if err := fn(s.WithTx(tx)); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("tx err: %w, rollback err: %v", err, rbErr)
		}
		return err
	}
	return tx.Commit()
}

// GetSystemConfigMultiple fetches multiple system_config keys in one query.
// Missing keys are absent from the returned map (no error).
func (s *SQLStore) GetSystemConfigMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	if len(keys) == 0 {
		return map[string]string{}, nil
	}
	placeholders := make([]string, len(keys))
	args := make([]any, len(keys))
	for i, k := range keys {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = k
	}
	query := "SELECT key, value FROM system_config WHERE key IN (" + strings.Join(placeholders, ", ") + ")"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]string, len(keys))
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		result[k] = v
	}
	return result, rows.Err()
}
