package couchtv

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"couchverse/internal/version"
)

// How the bulletins treat their sources: every request names the app, gives up after
// fetchTimeout and is tried again at most fetchRetries times, and asks only for a change
// where the source says what it last sent (ETag, Last-Modified).
const (
	fetchTimeout  = 10 * time.Second
	fetchRetries  = 2
	fetchMaxBytes = 8 << 20 // ČHMÚ's state of warnings is about 2 MB
)

// bulletinAgent names the app, as the sources' terms ask of automated clients.
var bulletinAgent = "Couchverse/" + version.Version + " (self-hosted media server; Couch TV bulletins)"

// errFetch is a source that answered with an error status.
type errFetch struct {
	url    string
	status int
}

func (e *errFetch) Error() string { return fmt.Sprintf("%s answered %d", e.url, e.status) }

// fetcher gets the bulletins' sources politely. It keeps what a source last sent with
// its validators, so an unchanged source answers 304 and the copy is reused.
type fetcher struct {
	http *http.Client
	// retryWait is the pause before the first retry, doubled before the next
	retryWait time.Duration

	mu   sync.Mutex
	seen map[string]fetched
}

type fetched struct {
	etag, modified string
	body           []byte
}

func newFetcher(client *http.Client) *fetcher {
	if client == nil {
		client = &http.Client{}
	}
	return &fetcher{http: client, retryWait: 2 * time.Second, seen: map[string]fetched{}}
}

// get fetches url, trying again after a timeout, a dropped connection, a 429 or a 5xx.
func (f *fetcher) get(ctx context.Context, url string) ([]byte, error) {
	wait := f.retryWait
	for attempt := 0; ; attempt++ {
		body, err := f.once(ctx, url)
		var status *errFetch
		retry := err != nil && ctx.Err() == nil &&
			(!errors.As(err, &status) || status.status == http.StatusTooManyRequests || status.status >= 500)
		if !retry || attempt == fetchRetries {
			return body, err
		}
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return nil, ctx.Err()
		case <-t.C:
		}
		wait *= 2
	}
}

func (f *fetcher) once(ctx context.Context, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", bulletinAgent)
	f.mu.Lock()
	prev, known := f.seen[url]
	f.mu.Unlock()
	if known {
		if prev.etag != "" {
			req.Header.Set("If-None-Match", prev.etag)
		}
		if prev.modified != "" {
			req.Header.Set("If-Modified-Since", prev.modified)
		}
	}
	res, err := f.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotModified && known {
		return prev.body, nil
	}
	if res.StatusCode != http.StatusOK {
		return nil, &errFetch{url: url, status: res.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, fetchMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > fetchMaxBytes {
		return nil, fmt.Errorf("%s sent more than %d bytes", url, fetchMaxBytes)
	}
	next := fetched{etag: res.Header.Get("ETag"), modified: res.Header.Get("Last-Modified"), body: body}
	f.mu.Lock()
	if next.etag != "" || next.modified != "" {
		f.seen[url] = next
	} else {
		delete(f.seen, url)
	}
	f.mu.Unlock()
	return body, nil
}
