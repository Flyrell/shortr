package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultFilePath         = "/data/shortr.json"
	defaultFileInterval     = 10 * time.Second
	minimumFileInterval     = time.Second
	fileVersion             = 1
	fileDirectoryPermission = 0o750
)

var _ Adapter = (*File)(nil)

// File is the memory adapter with a snapshot on disk behind it: it reads one at
// startup and rewrites it whenever the links changed, so a restart keeps them.
type File struct {
	*Memory

	path     string
	interval time.Duration
	dirty    atomic.Bool

	stop      chan struct{}
	flusher   sync.WaitGroup
	closeOnce sync.Once
	closeErr  error
}

type fileEntry struct {
	Target    string    `json:"target"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type fileSnapshot struct {
	Version int                  `json:"version"`
	URLs    map[string]fileEntry `json:"urls"`
}

func NewFile(env Env) (*File, error) { return newFile(env, time.Now) }

func newFile(env Env, now clock) (*File, error) {
	path := fileVar(env, "FILE_PATH", defaultFilePath)
	interval, err := fileIntervalVar(env, "FILE_SNAPSHOT_INTERVAL", defaultFileInterval)
	if err != nil {
		return nil, err
	}
	urls, err := readSnapshot(path, now())
	if err != nil {
		return nil, err
	}
	// Writing the loaded links straight back turns an unwritable path into a
	// startup failure instead of a snapshot that quietly stops being updated.
	if err := writeSnapshot(path, urls); err != nil {
		return nil, err
	}
	memory := newMemory(now)
	memory.mu.Lock()
	memory.urls = urls
	memory.mu.Unlock()

	file := &File{Memory: memory, path: path, interval: interval, stop: make(chan struct{})}
	file.flusher.Add(1)
	go file.flush()
	return file, nil
}

func (f *File) SaveURL(ctx context.Context, code, target string, ttl time.Duration) error {
	if err := f.Memory.SaveURL(ctx, code, target, ttl); err != nil {
		return err
	}
	f.dirty.Store(true)
	return nil
}

func (f *File) Close() error {
	f.closeOnce.Do(func() {
		close(f.stop)
		f.flusher.Wait()
		f.closeErr = f.snapshot()
	})
	if err := f.Memory.Close(); err != nil {
		return err
	}
	return f.closeErr
}

func (f *File) flush() {
	defer f.flusher.Done()

	ticker := time.NewTicker(f.interval)
	defer ticker.Stop()

	for {
		select {
		case <-f.stop:
			return
		case <-ticker.C:
			if !f.dirty.Swap(false) {
				continue
			}
			if err := f.snapshot(); err != nil {
				f.dirty.Store(true)
				slog.Error("writing the url snapshot failed", "path", f.path, "error", err)
			}
		}
	}
}

func (f *File) snapshot() error {
	f.mu.Lock()
	urls := maps.Clone(f.urls)
	f.mu.Unlock()

	return writeSnapshot(f.path, urls)
}

func writeSnapshot(path string, urls map[string]urlEntry) error {
	entries := make(map[string]fileEntry, len(urls))
	for code, entry := range urls {
		entries[code] = fileEntry{Target: entry.target, ExpiresAt: entry.expiresAt}
	}
	data, err := json.Marshal(fileSnapshot{Version: fileVersion, URLs: entries})
	if err != nil {
		return fmt.Errorf("file: encode the snapshot: %w", err)
	}
	return writeFile(path, data)
}

// The bytes reach the disk under a temporary name and are published with a
// rename, so an interrupted write leaves the previous snapshot whole.
func writeFile(path string, data []byte) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, fileDirectoryPermission); err != nil {
		return fmt.Errorf("file: create %q: %w", directory, err)
	}
	temporary, err := os.CreateTemp(directory, filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("file: create a temporary file in %q: %w", directory, err)
	}
	if err = writeAndSync(temporary, data); err == nil {
		err = os.Rename(temporary.Name(), path)
	}
	if err != nil {
		return errors.Join(fmt.Errorf("file: write %q: %w", path, err), os.Remove(temporary.Name()))
	}
	return nil
}

func writeAndSync(file *os.File, data []byte) error {
	if _, err := file.Write(data); err != nil {
		return errors.Join(err, file.Close())
	}
	if err := file.Sync(); err != nil {
		return errors.Join(err, file.Close())
	}
	return file.Close()
}

func readSnapshot(path string, now time.Time) (map[string]urlEntry, error) {
	// The path is operator configuration, not anything a request can reach.
	data, err := os.ReadFile(path) //nolint:gosec // the path comes from FILE_PATH
	if errors.Is(err, fs.ErrNotExist) {
		return make(map[string]urlEntry), nil
	}
	if err != nil {
		return nil, fmt.Errorf("file: read %q: %w", path, err)
	}
	var snapshot fileSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("file: %q is not a snapshot: %w", path, err)
	}
	if snapshot.Version != fileVersion {
		return nil, fmt.Errorf("file: %q has snapshot version %d, want %d", path, snapshot.Version, fileVersion)
	}
	urls := make(map[string]urlEntry, len(snapshot.URLs))
	for code, entry := range snapshot.URLs {
		if expired(entry.ExpiresAt, now) {
			continue
		}
		urls[code] = urlEntry{target: entry.Target, expiresAt: entry.ExpiresAt}
	}
	return urls, nil
}

func fileVar(env Env, name, fallback string) string {
	if value, ok := env(name); ok {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return fallback
}

func fileIntervalVar(env Env, name string, fallback time.Duration) (time.Duration, error) {
	raw := fileVar(env, name, "")
	if raw == "" {
		return fallback, nil
	}
	interval, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("file: %s must be a duration such as 10s or 2m, got %q", name, raw)
	}
	if interval < minimumFileInterval {
		return 0, fmt.Errorf("file: %s must be at least %s, got %q", name, minimumFileInterval, raw)
	}
	return interval, nil
}
