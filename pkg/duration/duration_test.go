package duration_test

import (
	"testing"

	"github.com/pladd/token-lifetime-filtering-webhook/pkg/duration"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    int64
		wantErr bool
	}{
		{name: "empty defaults to 1y", in: "", want: 365 * 24 * 3600},
		{name: "seconds", in: "3600s", want: 3600},
		{name: "minutes", in: "60m", want: 3600},
		{name: "hours", in: "24h", want: 86400},
		{name: "days", in: "7d", want: 7 * 86400},
		{name: "weeks", in: "2w", want: 2 * 7 * 86400},
		{name: "years", in: "1y", want: 365 * 86400},
		{name: "compound", in: "1h30m", want: 5400},
		{name: "uppercase", in: "1H", want: 3600},
		{name: "with spaces", in: " 1h 30m ", want: 5400},
		{name: "invalid unit", in: "1x", wantErr: true},
		{name: "number only", in: "3600", wantErr: true},
		{name: "unit only", in: "h", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := duration.Parse(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %d", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}
