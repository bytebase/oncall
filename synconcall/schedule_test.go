package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseSchedule(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantErr     bool
		errContains string
		validate    func(t *testing.T, rotations []Rotation)
	}{
		{
			name: "valid schedule",
			content: `2026-01-12T00:00:00Z,d@bytebase.com,vh@bytebase.com
2026-02-09T00:00:00Z,vh@bytebase.com,xz@bytebase.com
2026-03-09T00:00:00Z,xz@bytebase.com,zp@bytebase.com`,
			wantErr: false,
			validate: func(t *testing.T, rotations []Rotation) {
				if len(rotations) != 3 {
					t.Errorf("expected 3 rotations, got %d", len(rotations))
				}
				if rotations[0].Primary != "d@bytebase.com" {
					t.Errorf("expected primary d@bytebase.com, got %s", rotations[0].Primary)
				}
				if rotations[0].Secondary != "vh@bytebase.com" {
					t.Errorf("expected secondary vh@bytebase.com, got %s", rotations[0].Secondary)
				}
			},
		},
		{
			name:        "empty file",
			content:     "",
			wantErr:     true,
			errContains: "empty",
		},
		{
			name:        "only whitespace",
			content:     "   \n  \n  ",
			wantErr:     true,
			errContains: "empty",
		},
		{
			name:        "wrong field count - too few",
			content:     "2026-01-12T00:00:00Z,d@bytebase.com",
			wantErr:     true,
			errContains: "expected 3 fields",
		},
		{
			name:        "wrong field count - too many",
			content:     "2026-01-12T00:00:00Z,d@bytebase.com,vh@bytebase.com,extra@bytebase.com",
			wantErr:     true,
			errContains: "expected 3 fields",
		},
		{
			name:        "malformed timestamp",
			content:     "2026-01-12,d@bytebase.com,vh@bytebase.com",
			wantErr:     true,
			errContains: "invalid timestamp format",
		},
		{
			name:        "invalid timestamp format",
			content:     "not-a-date,d@bytebase.com,vh@bytebase.com",
			wantErr:     true,
			errContains: "invalid timestamp format",
		},
		{
			name: "timestamps not in ascending order",
			content: `2026-02-09T00:00:00Z,vh@bytebase.com,xz@bytebase.com
2026-01-12T00:00:00Z,d@bytebase.com,vh@bytebase.com`,
			wantErr:     true,
			errContains: "not in ascending order",
		},
		{
			name: "duplicate timestamps",
			content: `2026-01-12T00:00:00Z,d@bytebase.com,vh@bytebase.com
2026-01-12T00:00:00Z,vh@bytebase.com,xz@bytebase.com`,
			wantErr:     true,
			errContains: "not in ascending order",
		},
		{
			name:        "invalid primary email - no @",
			content:     "2026-01-12T00:00:00Z,invalid-email,vh@bytebase.com",
			wantErr:     true,
			errContains: "invalid primary email",
		},
		{
			name:        "invalid secondary email - no domain",
			content:     "2026-01-12T00:00:00Z,d@bytebase.com,vh@",
			wantErr:     true,
			errContains: "invalid secondary email",
		},
		{
			name:        "invalid email - @ at start",
			content:     "2026-01-12T00:00:00Z,@bytebase.com,vh@bytebase.com",
			wantErr:     true,
			errContains: "invalid primary email",
		},
		{
			name:        "invalid email - @ at end",
			content:     "2026-01-12T00:00:00Z,d@bytebase.com,vh@",
			wantErr:     true,
			errContains: "invalid secondary email",
		},
		{
			name:        "empty email",
			content:     "2026-01-12T00:00:00Z,,vh@bytebase.com",
			wantErr:     true,
			errContains: "invalid primary email",
		},
		{
			name: "valid schedule with empty lines",
			content: `2026-01-12T00:00:00Z,d@bytebase.com,vh@bytebase.com

2026-02-09T00:00:00Z,vh@bytebase.com,xz@bytebase.com`,
			wantErr: false,
			validate: func(t *testing.T, rotations []Rotation) {
				if len(rotations) != 2 {
					t.Errorf("expected 2 rotations, got %d", len(rotations))
				}
			},
		},
		{
			name:    "valid schedule with whitespace around fields",
			content: "  2026-01-12T00:00:00Z  ,  d@bytebase.com  ,  vh@bytebase.com  ",
			wantErr: false,
			validate: func(t *testing.T, rotations []Rotation) {
				if rotations[0].Primary != "d@bytebase.com" {
					t.Errorf("expected trimmed primary, got %s", rotations[0].Primary)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "schedule.csv")
			if err := os.WriteFile(tmpFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}

			rotations, err := ParseSchedule(tmpFile)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.validate != nil {
					tt.validate(t, rotations)
				}
			}
		})
	}
}

func TestParseSchedule_FileNotFound(t *testing.T) {
	_, err := ParseSchedule("/nonexistent/file.csv")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
	if !containsString(err.Error(), "failed to open schedule file") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestFindCurrentRotation(t *testing.T) {
	// Fixed test rotations
	rotations := []Rotation{
		{
			StartTime: mustParseTime("2026-01-12T00:00:00Z"),
			Primary:   "d@bytebase.com",
			Secondary: "vh@bytebase.com",
		},
		{
			StartTime: mustParseTime("2026-02-09T00:00:00Z"),
			Primary:   "vh@bytebase.com",
			Secondary: "xz@bytebase.com",
		},
		{
			StartTime: mustParseTime("2026-03-09T00:00:00Z"),
			Primary:   "xz@bytebase.com",
			Secondary: "zp@bytebase.com",
		},
	}

	tests := []struct {
		name        string
		now         string
		wantPrimary string
		wantErr     bool
		errContains string
	}{
		{
			name:        "current time in first rotation",
			now:         "2026-01-15T12:00:00Z",
			wantPrimary: "d@bytebase.com",
			wantErr:     false,
		},
		{
			name:        "current time in second rotation",
			now:         "2026-02-20T12:00:00Z",
			wantPrimary: "vh@bytebase.com",
			wantErr:     false,
		},
		{
			name:        "current time in third rotation",
			now:         "2026-03-15T12:00:00Z",
			wantPrimary: "xz@bytebase.com",
			wantErr:     false,
		},
		{
			name:        "current time exactly at rotation boundary",
			now:         "2026-02-09T00:00:00Z",
			wantPrimary: "vh@bytebase.com",
			wantErr:     false,
		},
		{
			name:        "current time before first rotation",
			now:         "2026-01-01T00:00:00Z",
			wantErr:     true,
			errContains: "before first rotation start",
		},
		{
			name:        "current time after last rotation",
			now:         "2026-12-31T23:59:59Z",
			wantPrimary: "xz@bytebase.com",
			wantErr:     false,
		},
		{
			name:        "current time one second before rotation change",
			now:         "2026-02-08T23:59:59Z",
			wantPrimary: "d@bytebase.com",
			wantErr:     false,
		},
		{
			name:        "current time one second after rotation start",
			now:         "2026-02-09T00:00:01Z",
			wantPrimary: "vh@bytebase.com",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := mustParseTime(tt.now)
			rotation, err := FindCurrentRotation(rotations, now)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if rotation == nil {
					t.Fatal("expected rotation, got nil")
				}
				if rotation.Primary != tt.wantPrimary {
					t.Errorf("expected primary %s, got %s", tt.wantPrimary, rotation.Primary)
				}
			}
		})
	}
}

func TestFindCurrentRotation_EmptyRotations(t *testing.T) {
	now := mustParseTime("2026-01-15T12:00:00Z")
	_, err := FindCurrentRotation([]Rotation{}, now)
	if err == nil {
		t.Error("expected error for empty rotations")
	}
	if !containsString(err.Error(), "no rotations available") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateRotation(t *testing.T) {
	tests := []struct {
		name        string
		rotation    Rotation
		wantErr     bool
		errContains string
	}{
		{
			name: "valid rotation",
			rotation: Rotation{
				StartTime: time.Now(),
				Primary:   "d@bytebase.com",
				Secondary: "vh@bytebase.com",
			},
			wantErr: false,
		},
		{
			name: "invalid primary email",
			rotation: Rotation{
				StartTime: time.Now(),
				Primary:   "invalid",
				Secondary: "vh@bytebase.com",
			},
			wantErr:     true,
			errContains: "invalid primary email",
		},
		{
			name: "invalid secondary email",
			rotation: Rotation{
				StartTime: time.Now(),
				Primary:   "d@bytebase.com",
				Secondary: "invalid",
			},
			wantErr:     true,
			errContains: "invalid secondary email",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRotation(tt.rotation)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestIsValidEmail(t *testing.T) {
	tests := []struct {
		email string
		valid bool
	}{
		{"d@bytebase.com", true},
		{"user@example.org", true},
		{"test.user@sub.domain.com", true},
		{"invalid", false},
		{"@bytebase.com", false},
		{"user@", false},
		{"user", false},
		{"", false},
		{"user@domain", false}, // no dot after @
		{"user.domain.com", false}, // no @
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			result := isValidEmail(tt.email)
			if result != tt.valid {
				t.Errorf("isValidEmail(%q) = %v, want %v", tt.email, result, tt.valid)
			}
		})
	}
}

// Helper functions

func mustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func containsString(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
