-- +goose Up
ALTER TABLE transcode_variants
ADD COLUMN size_bytes bigint NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE transcode_variants
DROP COLUMN size_bytes;
