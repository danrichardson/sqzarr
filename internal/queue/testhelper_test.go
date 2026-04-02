//go:build integration

package queue_test

import (
	"log/slog"
	"testing"
)

func testLog(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.Default()
}
