package couchtv

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"time"

	"github.com/chromedp/cdproto/page"
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// browser is the sidecar's one headless Chromium with the render page open
// (window.couchtvRender, clients/broadcast/src/render.ts).
type browser struct {
	execPath string
	ctx      context.Context
	cancel   context.CancelFunc
}

// loadResult is what the render page's load reports.
type loadResult struct {
	Duration float64 `json:"duration"`
	// SoundDuration is how long the clip's sound lasts as an item of a break
	SoundDuration float64  `json:"soundDuration"`
	Version       int      `json:"version"`
	Missing       []string `json:"missing"`
}

// callTimeout bounds one call into the page: a frame, a capture, a clip's load with its
// artwork.
const callTimeout = 3 * time.Minute

// soundTimeout bounds loading and rendering a sound frames long. It grows with the sound:
// a dense break rendered in one pass ran about 5x slower than real time on a Pi (a 117 s
// part of 13 clips took 9 m 53 s), and an item alone, though far faster, gets the same room.
func soundTimeout(frames int) time.Duration {
	return callTimeout + 10*time.Duration(frames)*time.Second/FPS
}

// startTimeout bounds Chromium's start: on a Pi with a transcode on every core it takes
// well past chromedp's defaults (20 s to print its address, 10 s to answer).
const startTimeout = 2 * time.Minute

// open starts Chromium on the render page. On Linux it renders on the GPU when the
// container has one (/dev/dri, the Pi's V3D; flags from the planning benchmark) and runs
// without Chromium's sandbox, which needs privileges a container lacks; the page is the
// sidecar's own bundle on localhost, never web content.
func (b *browser) open(pageURL string) error {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.WindowSize(1920, 1080),
		chromedp.Flag("force-device-scale-factor", "1"),
		chromedp.Flag("run-all-compositor-stages-before-draw", true),
		chromedp.WSURLReadTimeout(startTimeout),
	)
	if b.execPath != "" {
		opts = append(opts, chromedp.ExecPath(b.execPath))
	}
	if runtime.GOOS == "linux" {
		opts = append(opts, chromedp.NoSandbox)
		if _, err := os.Stat("/dev/dri"); err == nil {
			opts = append(opts, chromedp.Flag("use-angle", "gles-egl"), chromedp.Flag("ignore-gpu-blocklist", true),
				chromedp.Flag("enable-gpu-rasterization", true), chromedp.Flag("enable-zero-copy", true))
		}
	}
	alloc, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	// chromedp's protocol types trail the browser, so it reports events it cannot read
	ctx, cancel := chromedp.NewContext(alloc, chromedp.WithBrowserOption(chromedp.WithDialTimeout(startTimeout)),
		chromedp.WithErrorf(func(format string, args ...any) {
			slog.Debug("chromium: " + fmt.Sprintf(format, args...))
		}))
	b.ctx = ctx
	b.cancel = func() {
		cancel()
		cancelAlloc()
	}
	// the browser lives as long as the context of the first run, so that one is b.ctx
	err := chromedp.Run(ctx)
	if err == nil {
		err = b.run(context.Background(), time.Minute,
			chromedp.EmulateViewport(1920, 1080),
			chromedp.Navigate(pageURL),
			chromedp.Poll(`!!window.couchtvRender`, nil, chromedp.WithPollingTimeout(time.Minute)))
	}
	if err != nil {
		b.close()
		return fmt.Errorf("chromium: %w", err)
	}
	return nil
}

func (b *browser) close() {
	if b.cancel != nil {
		b.cancel()
		b.cancel = nil
	}
}

// run runs actions in the page, giving up when ctx (a job's) ends or after timeout.
func (b *browser) run(ctx context.Context, timeout time.Duration, actions ...chromedp.Action) error {
	runCtx, cancel := context.WithTimeout(b.ctx, timeout)
	defer cancel()
	stop := context.AfterFunc(ctx, cancel)
	defer stop()
	err := chromedp.Run(runCtx, actions...)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

// eval evaluates an expression in the page, awaiting a promise, into out.
func (b *browser) eval(ctx context.Context, js string, out any) error {
	return b.evalWithin(ctx, callTimeout, js, out)
}

func (b *browser) evalWithin(ctx context.Context, timeout time.Duration, js string, out any) error {
	return b.run(ctx, timeout, chromedp.Evaluate(js, out, func(p *cdpruntime.EvaluateParams) *cdpruntime.EvaluateParams {
		return p.WithAwaitPromise(true)
	}))
}

func (b *browser) versions(ctx context.Context) (map[string]int, error) {
	var v map[string]int
	return v, b.eval(ctx, `window.couchtvRender.versions()`, &v)
}

// load loads a clip or break spec.
func (b *browser) load(ctx context.Context, spec []byte, timeout time.Duration) (loadResult, error) {
	var res loadResult
	return res, b.evalWithin(ctx, timeout, fmt.Sprintf(`window.couchtvRender.load(%s)`, spec), &res)
}

// loadSound loads a clip spec for its sound alone (laid out, never shown).
func (b *browser) loadSound(ctx context.Context, spec []byte, timeout time.Duration) (loadResult, error) {
	var res loadResult
	return res, b.evalWithin(ctx, timeout, fmt.Sprintf(`window.couchtvRender.load(%s, { sound: true })`, spec), &res)
}

// frame sets up the frame at index i for the next capture, unless nothing moves since the
// one before it; it reports whether the picture changed. It does not wait for a paint: the
// capture draws a fresh frame of its own, so waiting would draw every frame twice.
func (b *browser) frame(ctx context.Context, i int) (bool, error) {
	from := -1.0
	if i > 0 {
		from = frameSeconds(i - 1)
	}
	var changed bool
	return changed, b.eval(ctx, fmt.Sprintf(`window.couchtvRender.advance(%g, %g)`, frameSeconds(i), from), &changed)
}

// keyRects are the squeeze's program rectangles of the loaded clip's frames (nil where the
// panel is gone).
func (b *browser) keyRects(ctx context.Context, frames int) ([]*KeyRect, error) {
	var rects []*KeyRect
	return rects, b.eval(ctx, fmt.Sprintf(`Array.from({ length: %d }, (_, i) => window.couchtvRender.keyRect(i / %d))`, frames, FPS), &rects)
}

// capture is the page as a JPEG at the quality the plan measured (q92).
func (b *browser) capture(ctx context.Context) ([]byte, error) {
	var buf []byte
	err := b.run(ctx, callTimeout, chromedp.ActionFunc(func(ctx context.Context) error {
		var err error
		buf, err = page.CaptureScreenshot().WithFormat(page.CaptureScreenshotFormatJpeg).WithQuality(92).
			WithOptimizeForSpeed(true).WithFromSurface(true).Do(ctx)
		return err
	}))
	return buf, err
}

// renderAudio renders the loaded clip's sound and has the page post the WAV to url (a
// long WAV is too big to move as a string over CDP).
func (b *browser) renderAudio(ctx context.Context, url string, timeout time.Duration) error {
	return b.postAudio(ctx, "renderAudio", url, timeout)
}

// renderItemAudio has the page post the loaded clip's sound as an item of a break to url.
func (b *browser) renderItemAudio(ctx context.Context, url string, timeout time.Duration) error {
	return b.postAudio(ctx, "renderItemAudio", url, timeout)
}

func (b *browser) postAudio(ctx context.Context, method, url string, timeout time.Duration) error {
	target, _ := json.Marshal(url)
	var status int
	js := fmt.Sprintf(`(async (r) => {
		const wav = await r.%s();
		return (await fetch(%s, { method: 'POST', body: wav })).status;
	})(window.couchtvRender)`, method, target)
	if err := b.evalWithin(ctx, timeout, js, &status); err != nil {
		return err
	}
	if status != 204 {
		return fmt.Errorf("audio upload answered %d", status)
	}
	return nil
}
