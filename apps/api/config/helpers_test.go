package config

import (
	"errors"
	"strings"
	"testing"
)

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

func assertVarError(t *testing.T, err error, variable string) {
	t.Helper()

	var varErr *varError
	if !errors.As(err, &varErr) {
		t.Fatalf("Load() error = %v, want *varError", err)
	}
	if varErr.Variable != variable {
		t.Errorf("Variable = %q, want %q", varErr.Variable, variable)
	}
	if !strings.Contains(varErr.Error(), variable) {
		t.Errorf("Error() = %q, want it to name %q", varErr.Error(), variable)
	}
}
