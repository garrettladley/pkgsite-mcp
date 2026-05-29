package tools

import (
	"strconv"
	"testing"
)

func TestUnexpectedPkgsiteStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status int
		want   bool
	}{
		{status: 400, want: false},
		{status: 404, want: false},
		{status: 429, want: false},
		{status: 500, want: true},
		{status: 503, want: true},
	}

	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.status), func(t *testing.T) {
			t.Parallel()

			if got := unexpectedPkgsiteStatus(tt.status); got != tt.want {
				t.Fatalf("unexpectedPkgsiteStatus(%d) = %t, want %t", tt.status, got, tt.want)
			}
		})
	}
}
