package format

import "testing"

func TestHumanizeBytes(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{name: "zero bytes", bytes: 0, want: "0 B"},
		{name: "single byte", bytes: 1, want: "1 B"},
		{name: "under 1KB", bytes: 1023, want: "1023 B"},
		{name: "exactly 1KB", bytes: 1024, want: "1.0 KB"},
		{name: "1.5 KB", bytes: 1536, want: "1.5 KB"},
		{name: "exactly 1MB", bytes: 1024 * 1024, want: "1.0 MB"},
		{name: "50 MB", bytes: 50 * 1024 * 1024, want: "50.0 MB"},
		{name: "exactly 1GB", bytes: 1024 * 1024 * 1024, want: "1.0 GB"},
		{name: "1.5 GB", bytes: 1536 * 1024 * 1024, want: "1.5 GB"},
		{name: "exactly 1TB", bytes: 1024 * 1024 * 1024 * 1024, want: "1.0 TB"},
		{name: "large value", bytes: 5 * 1024 * 1024 * 1024, want: "5.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HumanizeBytes(tt.bytes)
			if got != tt.want {
				t.Errorf("HumanizeBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}