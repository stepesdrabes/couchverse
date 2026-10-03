-- +goose Up
-- Playback v2 decides per device, so the probe records what devices care about:
-- codec tag, profile, level, bit depth, frame rate and the HDR format, with the
-- Dolby Vision profile and its base-layer compatibility. probe_version marks rows
-- an older prober read; a background job probes them again.
ALTER TABLE media_files
    ADD COLUMN video_codec_tag text NOT NULL DEFAULT '',
    ADD COLUMN video_profile text NOT NULL DEFAULT '',
    ADD COLUMN video_level numeric NOT NULL DEFAULT 0,
    ADD COLUMN bit_depth int NOT NULL DEFAULT 0,
    ADD COLUMN frame_rate numeric NOT NULL DEFAULT 0,
    ADD COLUMN hdr_format text NOT NULL DEFAULT 'sdr'
        CHECK (hdr_format IN ('sdr', 'hdr10', 'hdr10plus', 'hlg', 'dolbyVision')),
    ADD COLUMN dovi_profile int NOT NULL DEFAULT 0,
    ADD COLUMN dovi_compatibility int NOT NULL DEFAULT 0,
    ADD COLUMN probe_version int NOT NULL DEFAULT 0;

ALTER TABLE audio_streams
    ADD COLUMN profile text NOT NULL DEFAULT '',
    ADD COLUMN channel_layout text NOT NULL DEFAULT '',
    ADD COLUMN sample_rate int NOT NULL DEFAULT 0;

-- Variants prepared before HLS v2 are MPEG-TS with the audio muxed in; they keep
-- playing through the legacy master until they are prepared again as fMP4.
ALTER TABLE transcode_variants
    ADD COLUMN format text NOT NULL DEFAULT 'ts' CHECK (format IN ('ts', 'fmp4'));
ALTER TABLE transcode_variants ALTER COLUMN format SET DEFAULT 'fmp4';

-- +goose Down
ALTER TABLE transcode_variants DROP COLUMN format;
ALTER TABLE audio_streams
    DROP COLUMN sample_rate,
    DROP COLUMN channel_layout,
    DROP COLUMN profile;
ALTER TABLE media_files
    DROP COLUMN probe_version,
    DROP COLUMN dovi_compatibility,
    DROP COLUMN dovi_profile,
    DROP COLUMN hdr_format,
    DROP COLUMN frame_rate,
    DROP COLUMN bit_depth,
    DROP COLUMN video_level,
    DROP COLUMN video_profile,
    DROP COLUMN video_codec_tag;
