package adapters

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	bolt "go.etcd.io/bbolt"
)

func TestFileKeepsLinksAcrossReopen(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	clock := newTestClock()
	path := filepath.Join(t.TempDir(), "shortr.db")
	overrides := map[string]string{"FILE_PATH": path}

	first := newFileAdapter(t, clock.Now, overrides)
	if err := first.SaveURL(ctx, "abc1234defgh", "https://example.com", time.Hour); err != nil {
		t.Fatalf("SaveURL() error = %v", err)
	}
	if err := first.SaveURL(ctx, "zyx9876wvuts", "https://short.example", time.Minute); err != nil {
		t.Fatalf("SaveURL() error = %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	clock.Advance(30 * time.Minute)

	second := newFileAdapter(t, clock.Now, overrides)
	got, err := second.FindURL(ctx, "abc1234defgh")
	if err != nil {
		t.Fatalf("FindURL() error = %v", err)
	}
	if want := "https://example.com"; got != want {
		t.Errorf("FindURL() = %q, want %q", got, want)
	}
	if _, err := second.FindURL(ctx, "zyx9876wvuts"); !errors.Is(err, ErrNotFound) {
		t.Errorf("FindURL() error = %v, want ErrNotFound for the expired code", err)
	}
}

func TestFileRejectsATakenCodeUntilItExpires(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	clock := newTestClock()
	adapter := newFileAdapter(t, clock.Now, nil)

	if err := adapter.SaveURL(ctx, "abc1234defgh", "https://example.com", time.Hour); err != nil {
		t.Fatalf("SaveURL() error = %v", err)
	}
	if err := adapter.SaveURL(ctx, "abc1234defgh", "https://other.example", time.Hour); !errors.Is(err, ErrCodeTaken) {
		t.Fatalf("SaveURL() error = %v, want ErrCodeTaken", err)
	}
	clock.Advance(2 * time.Hour)
	if err := adapter.SaveURL(ctx, "abc1234defgh", "https://other.example", time.Hour); err != nil {
		t.Fatalf("SaveURL() after expiry error = %v", err)
	}
}

func TestFileReadsDoNotDeleteExpiredRecords(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	clock := newTestClock()
	adapter := newFileAdapter(t, clock.Now, nil)

	if err := adapter.SaveURL(ctx, "abc1234defgh", "https://example.com", time.Hour); err != nil {
		t.Fatalf("SaveURL() error = %v", err)
	}
	clock.Advance(2 * time.Hour)
	if _, err := adapter.FindURL(ctx, "abc1234defgh"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("FindURL() error = %v, want ErrNotFound", err)
	}
	// The read filters the expired record but leaves removal to the sweeper.
	if count := storedCount(t, adapter); count != 1 {
		t.Errorf("stored records = %d, want the expired one still present", count)
	}
	if err := adapter.purge(); err != nil {
		t.Fatalf("purge() error = %v", err)
	}
	if count := storedCount(t, adapter); count != 0 {
		t.Errorf("stored records = %d, want the sweep to have removed it", count)
	}
}

func TestFileSweepsExpiredRecordsOnItsTicker(t *testing.T) {
	// The bubble drives the sweep ticker on a fake clock, so the entry expires
	// and is swept without waiting on the wall clock.
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()
		adapter := newFileAdapter(t, time.Now, nil)

		if err := adapter.SaveURL(ctx, "abc1234defgh", "https://example.com", 30*time.Second); err != nil {
			t.Fatalf("SaveURL() error = %v", err)
		}
		time.Sleep(2 * sweepInterval)
		synctest.Wait()

		if count := storedCount(t, adapter); count != 0 {
			t.Errorf("stored records = %d, want the ticker to have emptied them", count)
		}
	})
}

func TestNewFileReportsAnUnreadableDatabase(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "shortr.db")
	if err := os.WriteFile(path, []byte("not a bolt database"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	adapter, err := NewFile(envFrom(map[string]string{"FILE_PATH": path}))
	if err == nil {
		t.Fatalf("NewFile() = %v, want an error", adapter)
	}
	if adapter != nil {
		t.Errorf("NewFile() = %v, want nil", adapter)
	}
}

func TestNewFileReportsAnUnwritablePath(t *testing.T) {
	t.Parallel()

	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, nil, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	path := filepath.Join(blocked, "shortr.db")

	adapter, err := NewFile(envFrom(map[string]string{"FILE_PATH": path}))
	if err == nil {
		t.Fatalf("NewFile() = %v, want an error", adapter)
	}
	if adapter != nil {
		t.Errorf("NewFile() = %v, want nil", adapter)
	}
}

func TestNewFileRefusesASecondOpener(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "shortr.db")
	first := newFileAdapter(t, time.Now, map[string]string{"FILE_PATH": path})
	_ = first

	adapter, err := NewFile(envFrom(map[string]string{"FILE_PATH": path}))
	if err == nil {
		t.Fatalf("NewFile() = %v, want an error while the database is held open", adapter)
	}
	if adapter != nil {
		t.Errorf("NewFile() = %v, want nil", adapter)
	}
}

func storedCount(t *testing.T, adapter *File) int {
	t.Helper()

	var count int
	if err := adapter.db.View(func(tx *bolt.Tx) error {
		count = tx.Bucket(urlsBucket).Stats().KeyN
		return nil
	}); err != nil {
		t.Fatalf("View() error = %v", err)
	}
	return count
}
