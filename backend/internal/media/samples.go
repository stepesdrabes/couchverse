package media

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"strings"
)

// Samples decodes a stretch of one audio stream of a file as mono 16 kHz and hands it to fn in
// chunks, for measurements that need the sound itself (where someone speaks). The stream is
// an ffmpeg stream specifier ("0:2"); empty reads the first audio track. A zero length reads
// to the end.
func Samples(ctx context.Context, ffmpegPath, file, stream string, from, length float64, fn func([]float32)) error {
	if stream == "" {
		stream = "0:a:0"
	}
	args := append([]string{"-nostats", "-loglevel", "error"}, seekArgs(file, from, length)...)
	cmd := analysisCmd(ctx, ffmpegPath, append(args, "-map", stream, "-vn", "-sn", "-dn", "-ac", "1", "-ar", "16000",
		"-f", "s16le", "-")...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	raw := make([]byte, 2*16000)
	chunk := make([]float32, 16000)
	for {
		n, err := io.ReadFull(stdout, raw)
		for i := 0; i < n/2; i++ {
			chunk[i] = float32(int16(binary.LittleEndian.Uint16(raw[2*i:]))) / 32768
		}
		if n >= 2 {
			fn(chunk[:n/2])
		}
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
				return err
			}
			break
		}
	}
	if err := cmd.Wait(); err != nil {
		return analysisError(ctx, err, stderr.String())
	}
	return nil
}
