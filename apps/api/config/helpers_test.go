package config

import "testing"

func lookupFrom(values map[string]string) Lookup {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}

func withStaticDir(t *testing.T, values map[string]string) map[string]string {
	t.Helper()

	merged := map[string]string{"STATIC_DIR": t.TempDir()}
	for key, value := range values {
		merged[key] = value
	}
	return merged
}
