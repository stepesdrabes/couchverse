// Package catalog owns titles, seasons, episodes, genres, browse/search and
// per-user progress + watchlist.
package catalog

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	db *pgxpool.Pool
	// RandomPlaybackChanged hears that an admin changed whether a title's episodes may
	// play in random order, which Couch TV's strips follow; nil for nobody.
	RandomPlaybackChanged func(ctx context.Context) error
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}
