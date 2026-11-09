package services

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeTaskTitle(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "trims surrounding whitespae",
			input: "\tFinish task\n",
			want:  "Finish task",
		},
		{
			name:    "rejects whitespace only",
			input:   " \t\n",
			wantErr: true,
		},
		{
			name:  "accepts 200 unicode code points",
			input: strings.Repeat("界", 200),
			want:  strings.Repeat("界", 200),
		},
		{
			name:    "rejects 201 unicode code points",
			input:   strings.Repeat("界", 201),
			wantErr: true,
		},
		{
			name:    "rejects invalid UTF-8",
			input:   string([]byte{0xff}),
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeTaskTitle(tc.input)

			if tc.wantErr {
				if !errors.Is(err, ErrInvalidTaskInput) {
					t.Fatalf("expected invalid input error, got %v", err)
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}

			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}
