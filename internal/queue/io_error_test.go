package queue

import "testing"

func TestIsIOError(t *testing.T) {
	tests := []struct {
		msg  string
		want bool
	}{
		// I/O errors — should return true
		{"open /tmp/output.mkv: permission denied", true},
		{"Permission denied", true},
		{"write /media/tv/file.mkv: read-only file system", true},
		{"fallocate: no space left on device", true},
		{"disk full", true},
		{"read /dev/sda1: input/output error", true},
		{"I/O error during write", true},
		{"rename: operation not permitted", true},
		{"open: is a directory", true},
		{"accept: too many open files", true},
		{"Access is denied", true},

		// Non-I/O errors — should return false
		{"exit status 1", false},
		{"signal: killed", false},
		{"codec not supported", false},
		{"verify failed: duration mismatch", false},
		{"rename output: file exists", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			got := isIOError(tt.msg)
			if got != tt.want {
				t.Errorf("isIOError(%q) = %v, want %v", tt.msg, got, tt.want)
			}
		})
	}
}
