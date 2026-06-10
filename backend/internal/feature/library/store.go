// Package library owns media ingestion: libraries on disk, the scanner and
// prober pipeline, chunked uploads and prepared HLS variants.
package library

import (
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store owns the libraries, media_files, upload_sessions and
// transcode_variants SQL over the shared pool.
type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}
