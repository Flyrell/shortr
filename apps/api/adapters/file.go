package adapters

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
	bolterrors "go.etcd.io/bbolt/errors"
)

const (
	defaultFilePath         = "/data/shortr.db"
	fileOpenTimeout         = 2 * time.Second
	fileMode                = 0o600
	fileDirectoryPermission = 0o750
	// The value is an 8 byte big-endian expiry stamp followed by the target, so
	// a read compares the stamp without decoding the whole record.
	expiryWidth = 8
)

var (
	_ Adapter = (*File)(nil)

	urlsBucket = []byte("urls")
)

// File keeps every link in a bbolt database on disk: a lookup walks the B+tree
// for one code instead of holding the whole set in memory.
type File struct {
	db  *bolt.DB
	now clock

	stop      chan struct{}
	sweeper   sync.WaitGroup
	closeOnce sync.Once
	closeErr  error
}

func NewFile(env Env) (*File, error) { return newFile(env, time.Now) }

func newFile(env Env, now clock) (*File, error) {
	path := fileVar(env, "FILE_PATH", defaultFilePath)
	if err := os.MkdirAll(filepath.Dir(path), fileDirectoryPermission); err != nil {
		return nil, fmt.Errorf("file: create %q: %w", filepath.Dir(path), err)
	}
	// The open takes an exclusive lock, so a second process fails here rather
	// than corrupting the database; the timeout turns that into a prompt error.
	db, err := bolt.Open(path, fileMode, &bolt.Options{Timeout: fileOpenTimeout})
	if err != nil {
		return nil, fmt.Errorf("file: open %q: %w", path, err)
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(urlsBucket)
		return err
	}); err != nil {
		return nil, errors.Join(fmt.Errorf("file: prepare %q: %w", path, err), db.Close())
	}

	file := &File{db: db, now: now, stop: make(chan struct{})}
	file.sweeper.Add(1)
	go file.sweep()
	return file, nil
}

func (f *File) SaveURL(ctx context.Context, code, target string, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if ttl <= 0 {
		return errInvalidTTL
	}
	now := f.now()
	err := f.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(urlsBucket)
		if existing := bucket.Get([]byte(code)); existing != nil && !expiredRecord(existing, now) {
			return ErrCodeTaken
		}
		return bucket.Put([]byte(code), encodeRecord(target, now.Add(ttl)))
	})
	return f.mapErr(err)
}

func (f *File) FindURL(ctx context.Context, code string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	now := f.now()
	var target string
	err := f.db.View(func(tx *bolt.Tx) error {
		record := tx.Bucket(urlsBucket).Get([]byte(code))
		if record == nil || expiredRecord(record, now) {
			return ErrNotFound
		}
		target = string(record[expiryWidth:])
		return nil
	})
	if err != nil {
		return "", f.mapErr(err)
	}
	return target, nil
}

func (f *File) Ping(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return f.mapErr(f.db.View(func(*bolt.Tx) error { return nil }))
}

func (f *File) Close() error {
	f.closeOnce.Do(func() {
		close(f.stop)
		f.sweeper.Wait()
		f.closeErr = f.db.Close()
	})
	return f.closeErr
}

func (f *File) sweep() {
	defer f.sweeper.Done()

	ticker := time.NewTicker(sweepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-f.stop:
			return
		case <-ticker.C:
			// Expired records are filtered on read regardless, so a failed sweep
			// is logged and left for the next tick rather than stopping it.
			if err := f.purge(); err != nil {
				slog.Error("sweeping expired links failed", "error", err)
			}
		}
	}
}

func (f *File) purge() error {
	now := f.now()
	return f.mapErr(f.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(urlsBucket)
		var expired [][]byte
		if err := bucket.ForEach(func(code, record []byte) error {
			if expiredRecord(record, now) {
				expired = append(expired, append([]byte(nil), code...))
			}
			return nil
		}); err != nil {
			return err
		}
		for _, code := range expired {
			if err := bucket.Delete(code); err != nil {
				return err
			}
		}
		return nil
	}))
}

func (f *File) mapErr(err error) error {
	if errors.Is(err, bolterrors.ErrDatabaseNotOpen) {
		return errClosed
	}
	return err
}

func encodeRecord(target string, expiresAt time.Time) []byte {
	record := make([]byte, expiryWidth+len(target))
	binary.BigEndian.PutUint64(record, uint64(expiresAt.UnixNano()))
	copy(record[expiryWidth:], target)
	return record
}

func expiredRecord(record []byte, now time.Time) bool {
	if len(record) < expiryWidth {
		return true
	}
	// The stamp was written from a UnixNano int64, so the reverse cast round-trips it.
	expiresAt := int64(binary.BigEndian.Uint64(record)) //nolint:gosec // round-trips a UnixNano stamp
	return expiresAt <= now.UnixNano()
}

func fileVar(env Env, name, fallback string) string {
	if value, ok := env(name); ok {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return fallback
}
