package handler

import "testing"

func TestParseDateOrNil(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantNil   bool
		wantDate  string
		expectErr bool
	}{
		{name: "empty", input: "", wantNil: true},
		{name: "placeholder dash", input: "-", wantNil: true},
		{name: "date only", input: "2026-03-18", wantDate: "2026-03-18"},
		{name: "iso datetime", input: "2026-03-18T09:30:00Z", wantDate: "2026-03-18"},
		{name: "invalid text", input: "not-a-date", expectErr: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseDateOrNil(tc.input)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantNil {
				if got != nil {
					t.Fatalf("expected nil date, got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected parsed date, got nil")
			}
			if got.Format("2006-01-02") != tc.wantDate {
				t.Fatalf("expected date=%s, got=%s", tc.wantDate, got.Format("2006-01-02"))
			}
		})
	}
}
