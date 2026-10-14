package couchtv

// CouchTvTrailerScene is a scene of a title's trailer (D43): a stretch of one of its media
// files, picture and sound, cut where its lines begin and end.
type CouchTvTrailerScene struct {
	MediaFileID string `json:"mediaFileId" format:"uuid" doc:"The file the picture is cut from."`
	AudioFileID string `json:"audioFileId" format:"uuid" doc:"The file the sound is cut from: the picture's own, or an alternate file of the same movie or episode in the original language."`
	AudioStream *int   `json:"audioStream,omitempty" doc:"The audio stream's index (ffprobe's) in that file; absent takes its default track."`
	StartMs     int    `json:"startMs"`
	DurationMs  int    `json:"durationMs" doc:"Whole frames at 25 fps, 3 to 8 seconds."`
	// Lines are spoken in the scene, in order, relative to its start.
	Lines  []CouchTvTrailerLine `json:"lines"`
	Score  float64              `json:"score" doc:"How well it suits a trailer, 0 to 1."`
	Source string               `json:"source" enum:"subtitles,speech" doc:"How its lines were found: from the subtitles (all of them are now), or by listening (a picker before the subtitles-only one)."`
}

// CouchTvTrailerLine is a line spoken in a trailer's scene.
type CouchTvTrailerLine struct {
	FromMs int    `json:"fromMs"`
	ToMs   int    `json:"toMs"`
	Text   string `json:"text,omitempty" doc:"What is said, when the subtitles say; absent for a line found by listening."`
}
