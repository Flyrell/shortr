package main

import (
	"strings"
	"testing"
)

func TestRunReportsStartupFailures(t *testing.T) {
	tests := []struct {
		name   string
		env    map[string]string
		wantIn string
	}{
		{name: "missing static dir", env: map[string]string{"STATIC_DIR": "/nope"}, wantIn: "STATIC_DIR"},
		{name: "port out of range", env: map[string]string{"PORT": "0"}, wantIn: "PORT"},
		{name: "invalid ttl", env: map[string]string{"URL_TTL": "0d"}, wantIn: "URL_TTL"},
		{name: "unknown persister", env: map[string]string{"URL_PERSISTER": "postgres"}, wantIn: "postgres"},
		{name: "redis without credentials", env: map[string]string{"URL_PERSISTER": "redis"}, wantIn: "REDIS_HOST"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("STATIC_DIR", t.TempDir())
			for key, value := range test.env {
				t.Setenv(key, value)
			}

			err := run()
			if err == nil {
				t.Fatal("run() error = nil, want a startup failure")
			}
			if !strings.Contains(err.Error(), test.wantIn) {
				t.Errorf("run() error = %q, want it to name %q", err, test.wantIn)
			}
		})
	}
}
