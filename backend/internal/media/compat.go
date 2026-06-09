package media

// Direct-play compatibility: codecs/containers virtually every modern browser
// handles natively. hevc/av1-capable clients are handled at request time via
// the caps parameter, not here.

var videoContainers = map[string]bool{"mp4": true, "m4v": true, "webm": true}
var videoCodecs = map[string]bool{"h264": true, "vp9": true, "av1": true}
var audioInVideo = map[string]bool{"aac": true, "mp3": true, "opus": true, "vorbis": true, "flac": true, "": true}

// audio-only files
var audioContainers = map[string]bool{"mp3": true, "flac": true, "m4a": true, "ogg": true, "opus": true, "wav": true}
var audioCodecs = map[string]bool{"mp3": true, "aac": true, "flac": true, "opus": true, "vorbis": true, "pcm_s16le": true, "pcm_s24le": true}

func DirectPlay(p *ProbeResult) bool {
	if p.HasVideo {
		return videoContainers[p.Container] && videoCodecs[p.VideoCodec] && audioInVideo[p.AudioCodec]
	}
	return audioContainers[p.Container] && audioCodecs[p.AudioCodec]
}

var videoExts = map[string]bool{
	".mp4": true, ".m4v": true, ".mkv": true, ".webm": true, ".avi": true, ".mov": true, ".ts": true,
}

var audioExts = map[string]bool{
	".mp3": true, ".flac": true, ".m4a": true, ".ogg": true, ".opus": true, ".wav": true,
}

func IsVideoFile(ext string) bool { return videoExts[ext] }
func IsAudioFile(ext string) bool { return audioExts[ext] }
