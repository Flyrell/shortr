package adapters

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

func TestFileRestoresLinksAfterRestart(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	clock := newTestClock()
	path := filepath.Join(t.TempDir(), "shortr.json")
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
	// The startup write drops what the load dropped, so an expired link is gone
	// from the snapshot as well.
	if codes := snapshotCodes(t, path); len(codes) != 1 || codes[0] != "abc1234defgh" {
		t.Errorf("snapshot codes = %v, want only the live one", codes)
	}
}

func TestFileWritesOnItsTickerOnlyWhenLinksChanged(t *testing.T) {
	// The bubble gives the ticker a fake clock, so the flushes happen without
	// waiting on the wall clock.
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()
		path := filepath.Join(t.TempDir(), "shortr.json")
		adapter := newFileAdapter(t, time.Now, map[string]string{"FILE_PATH": path, "FILE_SNAPSHOT_INTERVAL": "1s"})

		// Nothing was saved yet, so the ticker must leave the removed file alone.
		if err := os.Remove(path); err != nil {
			t.Fatalf("Remove() error = %v", err)
		}
		time.Sleep(2 * time.Second)
		synctest.Wait()
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("Stat() error = %v, want the snapshot to stay unwritten", err)
		}

		if err := adapter.SaveURL(ctx, "abc1234defgh", "https://example.com", time.Hour); err != nil {
			t.Fatalf("SaveURL() error = %v", err)
		}
		time.Sleep(2 * time.Second)
		synctest.Wait()

		if codes := snapshotCodes(t, path); len(codes) != 1 || codes[0] != "abc1234defgh" {
			t.Errorf("snapshot codes = %v, want the saved code", codes)
		}
	})
}

func TestFileWritesOnClose(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "shortr.json")
	// An interval far beyond the test rules the ticker out of the result.
	adapter := newFileAdapter(t, time.Now, map[string]string{"FILE_PATH": path, "FILE_SNAPSHOT_INTERVAL": "1h"})
	if err := adapter.SaveURL(t.Context(), "abc1234defgh", "https://example.com", time.Hour); err != nil {
		t.Fatalf("SaveURL() error = %v", err)
	}
	if err := adapter.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if codes := snapshotCodes(t, path); len(codes) != 1 || codes[0] != "abc1234defgh" {
		t.Errorf("snapshot codes = %v, want the saved code", codes)
	}
}

func TestNewFileReportsEnvironmentErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		contents string
		override map[string]string
		want     string
	}{
		{name: "unparsable interval", override: map[string]string{"FILE_SNAPSHOT_INTERVAL": "soon"}, want: "FILE_SNAPSHOT_INTERVAL"},
		{name: "interval below the minimum", override: map[string]string{"FILE_SNAPSHOT_INTERVAL": "10ms"}, want: "FILE_SNAPSHOT_INTERVAL"},
		{name: "unreadable snapshot", contents: "{", want: "is not a snapshot"},
		{name: "unknown snapshot version", contents: `{"version":2,"urls":{}}`, want: "snapshot version 2"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			values := fileValues(t, test.override)
			if test.contents != "" {
				if err := os.WriteFile(values["FILE_PATH"], []byte(test.contents), 0o600); err != nil {
					t.Fatalf("WriteFile() error = %v", err)
				}
			}

			adapter, err := NewFile(envFrom(values))
			if err == nil {
				t.Fatalf("NewFile() = %v, want an error", adapter)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Errorf("NewFile() error = %q, want it to mention %q", err, test.want)
			}
			if adapter != nil {
				t.Errorf("NewFile() = %v, want nil", adapter)
			}
		})
	}
}

func TestNewFileReportsAnUnwritablePath(t *testing.T) {
	t.Parallel()

	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, nil, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	path := filepath.Join(blocked, "shortr.json")

	adapter, err := NewFile(envFrom(map[string]string{"FILE_PATH": path}))
	if err == nil {
		t.Fatalf("NewFile() = %v, want an error", adapter)
	}
	if !strings.Contains(err.Error(), path) && !strings.Contains(err.Error(), blocked) {
		t.Errorf("NewFile() error = %q, want it to name the path", err)
	}
}

func snapshotCodes(t *testing.T, path string) []string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var snapshot fileSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if snapshot.Version != fileVersion {
		t.Errorf("snapshot version = %d, want %d", snapshot.Version, fileVersion)
	}
	codes := make([]string, 0, len(snapshot.URLs))
	for code := range snapshot.URLs {
		codes = append(codes, code)
	}
	return codes
}
