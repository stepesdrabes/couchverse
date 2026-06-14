-- +goose Up
-- Multi-language audio, model B: a separate file per language linked to the same
-- title/episode. audio_role 'primary' is the normal full file; 'audio_alt' marks
-- an alternate-language sibling. audio_lang is the BCP-47-ish language code.
ALTER TABLE media_files ADD COLUMN audio_lang text NOT NULL DEFAULT '';
ALTER TABLE media_files ADD COLUMN audio_role text NOT NULL DEFAULT 'primary'
	CHECK (audio_role IN ('primary', 'audio_alt'));

-- +goose Down
ALTER TABLE media_files DROP COLUMN audio_role;
ALTER TABLE media_files DROP COLUMN audio_lang;
