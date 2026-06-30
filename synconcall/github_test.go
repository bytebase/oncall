package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadUserMapping(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "users.csv")
	content := `# comment line
d@bytebase.com,d-bytebase

xz@bytebase.com, RainbowDashy
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	mapping, err := LoadUserMapping(path)
	if err != nil {
		t.Fatalf("LoadUserMapping: %v", err)
	}
	if got, want := len(mapping), 2; got != want {
		t.Fatalf("mapping size = %d, want %d", got, want)
	}
	if got := mapping["d@bytebase.com"]; got != "d-bytebase" {
		t.Errorf("d@bytebase.com -> %q, want d-bytebase", got)
	}
	if got := mapping["xz@bytebase.com"]; got != "RainbowDashy" {
		t.Errorf("xz@bytebase.com -> %q, want RainbowDashy (trimmed)", got)
	}
}

func TestLoadUserMappingInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.csv")
	if err := os.WriteFile(path, []byte("only-one-field\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadUserMapping(path); err == nil {
		t.Fatal("expected error for malformed mapping, got nil")
	}
}

func TestGitHubTeamClientReverseMap(t *testing.T) {
	c := NewGitHubTeamClient("bytebase", "token", map[string]string{
		"d@bytebase.com":  "d-bytebase",
		"xz@bytebase.com": "RainbowDashy",
	})

	// usernameFor resolves a mapped email and passes through a raw login.
	if got := c.usernameFor("d@bytebase.com"); got != "d-bytebase" {
		t.Errorf("usernameFor(email) = %q, want d-bytebase", got)
	}
	if got := c.usernameFor("someone-manual"); got != "someone-manual" {
		t.Errorf("usernameFor(login) = %q, want passthrough", got)
	}

	// Reverse map is case-insensitive on the GitHub login.
	if got := c.userToEmail["rainbowdashy"]; got != "xz@bytebase.com" {
		t.Errorf("userToEmail[rainbowdashy] = %q, want xz@bytebase.com", got)
	}
}
