package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

const defaultLimit = 20
const maxLimit = 100

// parsePagination reads ?page=&limit= from the request, returning 1-based page
// and a clamped limit. Both default to sane values if absent or invalid.
func parsePagination(r *http.Request) (page, limit int32) {
	page = 1
	limit = defaultLimit
	if v := r.URL.Query().Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = int32(n)
		}
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if n > maxLimit {
				n = maxLimit
			}
			limit = int32(n)
		}
	}
	return
}

// nonNil returns the slice unchanged if non-nil, otherwise an empty slice.
// Prevents nil slices from marshaling as JSON null instead of [].
func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func parseUUID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("invalid UUID %q: %w", s, err)
	}
	return id, nil
}

// isUniqueViolation reports whether err is a Postgres unique-constraint error (23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
