package couchtv

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"couchverse/internal/feature/playback"
)

// packageTTL is how long a package read is reused: a rung may be added to it
// while it waits to air.
const packageTTL = 30 * time.Second

// DiskMedia reads the programs', breaks' and standby cards' packages under the
// data directory, and owns the slate.
type DiskMedia struct {
	dataDir string
	slate   *Slate

	mu       sync.Mutex
	packages map[string]cachedPackage
	breaks   map[string]*breakMedia
	fills    map[string]*fill
}

type cachedPackage struct {
	pkg  *playback.Package
	read time.Time
}

func NewDiskMedia(dataDir string, slate *Slate) *DiskMedia {
	return &DiskMedia{dataDir: dataDir, slate: slate, packages: map[string]cachedPackage{}, breaks: map[string]*breakMedia{},
		fills: map[string]*fill{}}
}

// Package reads a media file's package.
func (m *DiskMedia) Package(fileID string) (*playback.Package, error) {
	m.mu.Lock()
	c, ok := m.packages[fileID]
	m.mu.Unlock()
	if ok && time.Since(c.read) < packageTTL {
		return c.pkg, nil
	}
	pkg, err := playback.ReadPackage(m.dataDir, fileID)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.packages) > 512 {
		m.packages = map[string]cachedPackage{}
	}
	m.packages[fileID] = cachedPackage{pkg: pkg, read: time.Now()}
	return pkg, nil
}

// Break reads a break's assembled rendition; once ready it never changes.
func (m *DiskMedia) Break(path string) (*breakMedia, error) {
	m.mu.Lock()
	b, ok := m.breaks[path]
	m.mu.Unlock()
	if ok {
		return b, nil
	}
	b, err := readBreak(m.resolve(path))
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.breaks) > 512 {
		m.breaks = map[string]*breakMedia{}
	}
	m.breaks[path] = b
	return b, nil
}

// Fill is the slate for path "", else a standby card's package (a rendered clip,
// which never changes), to loop.
func (m *DiskMedia) Fill(path string) (*fill, error) {
	if path == "" {
		return m.slate.Fill(), nil
	}
	m.mu.Lock()
	f, ok := m.fills[path]
	m.mu.Unlock()
	if ok {
		return f, nil
	}
	dir := m.resolve(path)
	p, err := playback.ReadPackageDir(dir)
	if err != nil {
		return nil, err
	}
	if p.Source == nil || len(p.Stereo) == 0 {
		return nil, fmt.Errorf("couchtv: %s lacks its video or its sound", dir)
	}
	f = &fill{}
	if f.Video, err = openLoop(filepath.Join(dir, p.Source.Name), p.Source.Info); err != nil {
		return nil, err
	}
	if f.Audio, err = openLoop(filepath.Join(dir, p.Stereo[0].Name), p.Stereo[0].Info); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.fills) > 16 {
		m.fills = map[string]*fill{}
	}
	m.fills[path] = f
	return f, nil
}

// resolve turns a break's path (absolute, or relative to the data directory)
// into a directory.
func (m *DiskMedia) resolve(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(m.dataDir, path)
}

// readBreak reads a break's package, laid out like a media file's: the video in
// source/, the mix as AAC stereo.
func readBreak(dir string) (*breakMedia, error) {
	p, err := playback.ReadPackageDir(dir)
	if err != nil {
		return nil, err
	}
	if p.Source == nil || len(p.Source.Playlist.Segments) == 0 {
		return nil, fmt.Errorf("couchtv: %s holds no video rendition", dir)
	}
	b := &breakMedia{Video: p.Source}
	if len(p.Stereo) > 0 {
		b.Audio = &p.Stereo[0]
	}
	return b, nil
}
