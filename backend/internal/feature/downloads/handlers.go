// Package downloads prepares device-ready MP4s that native clients keep for
// offline viewing: one faststart file with the video the device decodes, the
// audio languages asked for and the subtitles as mov_text. Requests for the same
// plan share one prepared file; the hourly cleanup removes it after retention.
package downloads

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"couchverse/internal/db"
	"couchverse/internal/feature/auth"
	"couchverse/internal/feature/catalog"
	"couchverse/internal/feature/jobs"
	"couchverse/internal/feature/library"
	"couchverse/internal/feature/playback"
	"couchverse/internal/feature/subtitles"
	"couchverse/internal/grant"
	"couchverse/internal/httpx"
	"couchverse/internal/media"
	"couchverse/internal/settings"
)

// Retention is how long a prepared file stays on the server after it was last
// asked for; the device fetches it well within that.
const Retention = 72 * time.Hour

// JobType prepares one download file.
const JobType = "prepare_download"

// JobPayload names the file to prepare; mediaFileId keeps the source from being
// deleted after transcoding while the job still needs it.
type JobPayload struct {
	FileID      string `json:"fileId"`
	MediaFileID string `json:"mediaFileId"`
}

// Handlers serve the downloads API.
type Handlers struct {
	store     *Store
	catalog   *catalog.Store
	library   *library.Store
	subtitles *subtitles.Store
	jobs      *jobs.Store
	settings  *settings.Store
	grants    *grant.Signer
	ffmpeg    string
	dataDir   string
}

func NewHandlers(st *Store, cat *catalog.Store, lib *library.Store, subs *subtitles.Store, jb *jobs.Store, set *settings.Store, grants *grant.Signer, ffmpegPath, dataDir string) *Handlers {
	return &Handlers{store: st, catalog: cat, library: lib, subtitles: subs, jobs: jb, settings: set, grants: grants, ffmpeg: ffmpegPath, dataDir: dataDir}
}

// FilePath is where a prepared download lives.
func FilePath(dataDir, fileID string) string {
	return filepath.Join(dataDir, "cache", "downloads", fileID+".mp4")
}

// DownloadRequest asks for a movie or an episode to be prepared for offline viewing.
type DownloadRequest struct {
	Kind    string                 `json:"kind" enum:"movie,episode"`
	ID      string                 `json:"id" format:"uuid" doc:"The movie's title id or the episode id."`
	Quality string                 `json:"quality" enum:"original,1080p,720p,480p" doc:"original keeps the source picture (copied when the device decodes it, else the best rung); a rung transcodes to H.264 at that height unless the source is already no bigger."`
	Audio   []string               `json:"audio,omitempty" doc:"Audio languages to include, in order; absent or unmatched for the default track."`
	Profile playback.DeviceProfile `json:"profile" doc:"The device's capability profile, as for resolvePlayback."`
}

// Download is one of the user's downloads.
type Download struct {
	ID              string             `json:"id" format:"uuid"`
	Kind            string             `json:"kind" enum:"movie,episode"`
	TitleID         string             `json:"titleId" format:"uuid"`
	TitleSlug       string             `json:"titleSlug"`
	Title           string             `json:"title"`
	EpisodeID       string             `json:"episodeId,omitempty" format:"uuid"`
	SeasonNumber    int                `json:"seasonNumber,omitempty"`
	EpisodeNumber   int                `json:"episodeNumber,omitempty"`
	EpisodeName     string             `json:"episodeName,omitempty"`
	PosterID        *string            `json:"posterId"`
	PosterVer       int64              `json:"posterVer,omitempty"`
	BackdropID      *string            `json:"backdropId"`
	BackdropVer     int64              `json:"backdropVer,omitempty"`
	ThumbID         *string            `json:"thumbId"`
	ThumbVer        int64              `json:"thumbVer,omitempty"`
	Quality         string             `json:"quality" enum:"original,1080p,720p,480p"`
	Status          string             `json:"status" enum:"queued,preparing,ready,failed" doc:"failed can be asked for again with requestDownload."`
	Progress        int                `json:"progress" doc:"Preparation progress in percent."`
	SizeBytes       int64              `json:"sizeBytes,omitempty" doc:"The MP4's size once ready."`
	DurationSeconds float64            `json:"durationSeconds"`
	Height          int                `json:"height,omitempty" doc:"Picture height of a transcoded download; absent when the source video is kept."`
	Audio           []DownloadTrack    `json:"audio" doc:"Audio tracks in the MP4, the default first."`
	Subtitles       []DownloadSubtitle `json:"subtitles" doc:"Subtitle tracks in the MP4 (mov_text)."`
	URL             string             `json:"url,omitempty" doc:"The MP4 under a media grant, once ready; the grant expires, so fetch the download again for a fresh URL."`
	CreatedAt       time.Time          `json:"createdAt"`
	ExpiresAt       *time.Time         `json:"expiresAt" doc:"When the server deletes the prepared MP4 unless it is asked for again; null until ready."`
}

// DownloadTrack is an audio track of a download.
type DownloadTrack struct {
	Lang  string `json:"lang" doc:"BCP 47 language; und when unknown."`
	Label string `json:"label"`
}

// DownloadSubtitle is a subtitle track of a download.
type DownloadSubtitle struct {
	Lang   string `json:"lang"`
	Label  string `json:"label"`
	Forced bool   `json:"forced"`
}

// DownloadList is the user's downloads, newest first.
type DownloadList struct {
	Downloads []Download `json:"downloads"`
}

type requestInput struct {
	Body DownloadRequest
}

type downloadInput struct {
	ID string `path:"id" format:"uuid"`
}

type downloadOutput struct{ Body *Download }

type listOutput struct{ Body DownloadList }

// Request finds or plans the MP4, queues its preparation when needed and adds it
// to the user's downloads.
func (h *Handlers) Request(ctx context.Context, in *requestInput) (*downloadOutput, error) {
	user := auth.UserFrom(ctx)
	req := in.Body
	mf, titleID, episodeID, err := h.mediaFile(ctx, req)
	if err != nil {
		return nil, err
	}
	if mf.SourceDeletedAt != nil {
		return nil, httpx.Fail(http.StatusConflict, "source_deleted", "the original file was removed after transcoding, so there is nothing to download from")
	}
	tracks, err := h.library.AudioStreamsForFile(ctx, mf.ID)
	if err != nil {
		return nil, err
	}
	mp4Plan, err := playback.PlanDownload(req.Profile, mf, tracks, req.Quality, req.Audio, playback.DetectFeatures(h.ffmpeg))
	if errors.Is(err, playback.ErrNoDownload) {
		return nil, httpx.Fail(http.StatusUnprocessableEntity, "unsupported", "this device plays no download of this file")
	}
	if err != nil {
		return nil, err
	}
	subs, err := h.subtitles.SubtitlesForMediaFile(ctx, mf.ID)
	if err != nil {
		return nil, err
	}
	plan := Plan{Media: mp4Plan, Subtitles: []SubtitleRef{}}
	spec := mp4Plan.Spec() + ";s="
	for i, s := range subs {
		plan.Subtitles = append(plan.Subtitles, SubtitleRef{ID: s.ID, Lang: s.Lang, Label: s.Label, Forced: s.Forced})
		if i > 0 {
			spec += ","
		}
		spec += s.ID
	}

	file, inserted, err := h.store.UpsertFile(ctx, mf.ID, spec, plan)
	if err != nil {
		return nil, err
	}
	queue := inserted || file.Status == "failed"
	if file.Status == "ready" {
		// a file removed by hand is made again
		if _, serr := os.Stat(FilePath(h.dataDir, file.ID)); serr != nil {
			queue = true
		}
	}
	if queue {
		if !inserted {
			if _, err := h.store.Requeue(ctx, file.ID); err != nil {
				return nil, err
			}
		}
		if _, err := h.jobs.EnqueueJobOnce(ctx, JobType, JobPayload{FileID: file.ID, MediaFileID: mf.ID}, jobs.EnqueueOpts{}); err != nil {
			return nil, err
		}
	}
	id, err := h.store.AddDownload(ctx, user.ID, file.ID, titleID, episodeID, req.Quality)
	if err != nil {
		return nil, err
	}
	return h.one(ctx, user.ID, id)
}

// mediaFile resolves the request to the file to download: the title's or the
// episode's primary file, or an alternate-audio sibling (model B) when the first
// language asked for is only there.
func (h *Handlers) mediaFile(ctx context.Context, req DownloadRequest) (mf *media.MediaFile, titleID string, episodeID *string, err error) {
	switch req.Kind {
	case "movie":
		t, terr := h.catalog.TitleByID(ctx, req.ID)
		if terr != nil {
			return nil, "", nil, terr
		}
		titleID = t.ID
		mf, err = h.catalog.PrimaryMediaFileForTitle(ctx, req.ID)
	default:
		ref, rerr := h.catalog.EpisodeRef(ctx, req.ID)
		if rerr != nil {
			return nil, "", nil, httpx.ErrNotFound
		}
		titleID, episodeID = ref.TitleID, &ref.EpisodeID
		mf, err = h.catalog.PrimaryMediaFileForEpisode(ctx, req.ID)
	}
	if errors.Is(err, db.ErrNotFound) {
		return nil, "", nil, httpx.Fail(http.StatusNotFound, "no_media", "this title has no media file yet")
	}
	if err != nil || len(req.Audio) == 0 {
		return mf, titleID, episodeID, err
	}
	tracks, err := h.library.AudioStreamsForFile(ctx, mf.ID)
	if err != nil {
		return nil, "", nil, err
	}
	for _, t := range tracks {
		if media.SameLanguage(t.Lang, req.Audio[0]) {
			return mf, titleID, episodeID, nil
		}
	}
	siblings, err := h.catalog.AudioSiblings(ctx, mf.TitleID, mf.EpisodeID, mf.ID)
	if err != nil {
		return nil, "", nil, err
	}
	for i := range siblings {
		if media.SameLanguage(siblings[i].AudioLang, req.Audio[0]) {
			return &siblings[i], titleID, episodeID, nil
		}
	}
	return mf, titleID, episodeID, nil
}

func (h *Handlers) List(ctx context.Context, _ *struct{}) (*listOutput, error) {
	user := auth.UserFrom(ctx)
	rows, err := h.store.Downloads(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	out := DownloadList{Downloads: make([]Download, 0, len(rows))}
	for _, r := range rows {
		out.Downloads = append(out.Downloads, h.payload(r, user.ID))
	}
	return &listOutput{Body: out}, nil
}

func (h *Handlers) Get(ctx context.Context, in *downloadInput) (*downloadOutput, error) {
	return h.one(ctx, auth.UserFrom(ctx).ID, in.ID)
}

func (h *Handlers) one(ctx context.Context, userID int64, id string) (*downloadOutput, error) {
	r, err := h.store.Download(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	d := h.payload(r, userID)
	return &downloadOutput{Body: &d}, nil
}

// Delete removes the download from the user's list; the prepared file goes with
// the next cleanup once nobody else wants it.
func (h *Handlers) Delete(ctx context.Context, in *downloadInput) (*struct{}, error) {
	return nil, h.store.DeleteDownload(ctx, auth.UserFrom(ctx).ID, in.ID)
}

func (h *Handlers) payload(r *row, userID int64) Download {
	d := r.Download
	f := r.file
	d.Status, d.Progress, d.SizeBytes = f.Status, f.Progress, f.SizeBytes
	if !f.Plan.Media.Copy {
		d.Height = f.Plan.Media.Rendition.Height
	}
	d.Audio = make([]DownloadTrack, 0, len(f.Plan.Media.Audio))
	for _, a := range f.Plan.Media.Audio {
		d.Audio = append(d.Audio, DownloadTrack{Lang: media.BCP47(a.Track.Lang), Label: a.Track.Title})
	}
	d.Subtitles = make([]DownloadSubtitle, 0, len(f.Plan.Subtitles))
	for _, s := range f.Plan.Subtitles {
		d.Subtitles = append(d.Subtitles, DownloadSubtitle{Lang: s.Lang, Label: s.Label, Forced: s.Forced})
	}
	if f.Status == "ready" && f.ReadyAt != nil {
		expires := f.ReadyAt
		if f.RequestedAt.After(*expires) {
			expires = &f.RequestedAt
		}
		at := expires.Add(Retention)
		d.ExpiresAt = &at
		if g := h.grant(f.MediaFileID, userID); g != "" {
			d.URL = playback.MediaPath(g, "downloads/"+f.ID)
		}
	}
	return d
}

func (h *Handlers) grant(mediaFileID string, userID int64) string {
	resource, err := uuid.Parse(mediaFileID)
	if err != nil {
		return ""
	}
	return h.grants.Issue(grant.Grant{Scope: grant.Media, Subject: userID, Resource: resource}, grant.MediaTTL)
}

// Serve sends a prepared MP4 of the granted media file, with range support so
// an interrupted transfer resumes.
func (h *Handlers) Serve(w http.ResponseWriter, r *http.Request) {
	g, _ := grant.From(r.Context())
	id := httpx.UUID(r, "id")
	if id == "" {
		httpx.NotFound(w)
		return
	}
	f, err := h.store.FileByID(r.Context(), id)
	if err != nil {
		httpx.StoreErr(w, err)
		return
	}
	if f.MediaFileID != g.Resource.String() {
		httpx.NotFound(w)
		return
	}
	if f.Status != "ready" {
		httpx.Error(w, http.StatusConflict, "not_ready", "the download is still being prepared")
		return
	}
	file, err := os.Open(FilePath(h.dataDir, f.ID))
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "file_missing", "the prepared file is gone; request the download again")
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		httpx.Internal(w, err)
		return
	}
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Content-Disposition", `attachment; filename="`+f.ID+`.mp4"`)
	http.ServeContent(w, r, f.ID+".mp4", info.ModTime(), file)
}
