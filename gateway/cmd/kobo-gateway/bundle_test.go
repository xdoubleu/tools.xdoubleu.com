package main

import "testing"

func TestRunningInAppBundle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		execPath string
		want     bool
	}{
		{
			name:     "app bundle",
			execPath: "/Applications/KoboGateway.app/Contents/MacOS/kobo-gateway",
			want:     true,
		},
		{
			name:     "raw dev binary",
			execPath: "/Users/dev/gateway/bin/kobo-gateway-darwin-arm64",
			want:     false,
		},
		{
			name:     "empty path",
			execPath: "",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := runningInAppBundle(tt.execPath); got != tt.want {
				t.Errorf("runningInAppBundle(%q) = %v, want %v", tt.execPath, got, tt.want)
			}
		})
	}
}
