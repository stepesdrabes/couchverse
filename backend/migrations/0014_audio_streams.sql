-- +goose Up
-- Inventory of embedded audio tracks per file (model A). Populated by the
-- prober from ffprobe; drives the player's audio menu and the multi-audio HLS
-- remux (ffmpeg -var_stream_map). Descriptive metadata only - no files here.
CREATE TABLE audio_streams (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	media_file_id uuid NOT NULL REFERENCES media_files (id) ON DELETE CASCADE,
	stream_index int NOT NULL,
	codec text NOT NULL DEFAULT '',
	lang text NOT NULL DEFAULT 'und',
	label text NOT NULL DEFAULT '',
	channels int NOT NULL DEFAULT 0,
	is_default boolean NOT NULL DEFAULT false,
	created_at timestamptz NOT NULL DEFAULT now(),
	UNIQUE (media_file_id, stream_index)
);
CREATE INDEX audio_streams_media_file_idx ON audio_streams (media_file_id);

-- +goose Down
DROP TABLE audio_streams;
