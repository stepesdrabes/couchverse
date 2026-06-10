// Package catalog owns titles, seasons, episodes, genres, browse/search and
// per-user progress + watchlist.
package catalog

import (
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}
