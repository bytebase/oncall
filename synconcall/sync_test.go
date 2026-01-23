package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// MockGroupsClient is a mock implementation of GroupsClient for testing
type MockGroupsClient struct {
	members      map[string][]string // groupEmail -> list of members
	addedCalls   []string            // track added members
	removedCalls []string            // track removed members
	listError    error
	addError     error
	removeError  error
}

func NewMockGroupsClient() *MockGroupsClient {
	return &MockGroupsClient{
		members:      make(map[string][]string),
		addedCalls:   make([]string, 0),
		removedCalls: make([]string, 0),
	}
}

func (m *MockGroupsClient) ListMembers(groupEmail string) ([]string, error) {
	if m.listError != nil {
		return nil, m.listError
	}
	members, ok := m.members[groupEmail]
	if !ok {
		return []string{}, nil
	}
	// Return a copy to avoid mutation
	result := make([]string, len(members))
	copy(result, members)
	return result, nil
}

func (m *MockGroupsClient) AddMember(groupEmail, memberEmail string) error {
	if m.addError != nil {
		return m.addError
	}
	m.addedCalls = append(m.addedCalls, memberEmail)
	if m.members[groupEmail] == nil {
		m.members[groupEmail] = make([]string, 0)
	}
	m.members[groupEmail] = append(m.members[groupEmail], memberEmail)
	return nil
}

func (m *MockGroupsClient) RemoveMember(groupEmail, memberEmail string) error {
	if m.removeError != nil {
		return m.removeError
	}
	m.removedCalls = append(m.removedCalls, memberEmail)
	members := m.members[groupEmail]
	for i, member := range members {
		if member == memberEmail {
			m.members[groupEmail] = append(members[:i], members[i+1:]...)
			break
		}
	}
	return nil
}

func TestSync(t *testing.T) {
	// Create a test schedule file
	scheduleContent := `2026-01-12T00:00:00Z,d@bytebase.com,vh@bytebase.com
2026-02-09T00:00:00Z,vh@bytebase.com,xz@bytebase.com
2026-03-09T00:00:00Z,xz@bytebase.com,zp@bytebase.com`

	tmpDir := t.TempDir()
	scheduleFile := filepath.Join(tmpDir, "schedule.csv")
	if err := os.WriteFile(scheduleFile, []byte(scheduleContent), 0644); err != nil {
		t.Fatalf("failed to create schedule file: %v", err)
	}

	tests := []struct {
		name            string
		currentTime     string
		initialMembers  []string
		expectedAdded   []string
		expectedRemoved []string
		wantErr         bool
	}{
		{
			name:            "empty group - add both oncall",
			currentTime:     "2026-01-15T12:00:00Z",
			initialMembers:  []string{},
			expectedAdded:   []string{"d@bytebase.com", "vh@bytebase.com"},
			expectedRemoved: []string{},
			wantErr:         false,
		},
		{
			name:            "correct members already - no changes",
			currentTime:     "2026-01-15T12:00:00Z",
			initialMembers:  []string{"d@bytebase.com", "vh@bytebase.com"},
			expectedAdded:   []string{},
			expectedRemoved: []string{},
			wantErr:         false,
		},
		{
			name:            "partial overlap - remove old, add new",
			currentTime:     "2026-02-15T12:00:00Z",
			initialMembers:  []string{"d@bytebase.com", "vh@bytebase.com"},
			expectedAdded:   []string{"xz@bytebase.com"},
			expectedRemoved: []string{"d@bytebase.com"},
			wantErr:         false,
		},
		{
			name:            "complete mismatch - remove all, add both",
			currentTime:     "2026-03-15T12:00:00Z",
			initialMembers:  []string{"d@bytebase.com", "vh@bytebase.com"},
			expectedAdded:   []string{"xz@bytebase.com", "zp@bytebase.com"},
			expectedRemoved: []string{"d@bytebase.com", "vh@bytebase.com"},
			wantErr:         false,
		},
		{
			name:            "extra members in group - remove them",
			currentTime:     "2026-01-15T12:00:00Z",
			initialMembers:  []string{"d@bytebase.com", "vh@bytebase.com", "old@bytebase.com", "extra@bytebase.com"},
			expectedAdded:   []string{},
			expectedRemoved: []string{"old@bytebase.com", "extra@bytebase.com"},
			wantErr:         false,
		},
		{
			name:            "rotation change at boundary",
			currentTime:     "2026-02-09T00:00:00Z",
			initialMembers:  []string{"d@bytebase.com", "vh@bytebase.com"},
			expectedAdded:   []string{"xz@bytebase.com"},
			expectedRemoved: []string{"d@bytebase.com"},
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockGroupsClient()
			groupEmail := "dev-oncall@bytebase.com"

			// Set up initial members
			mock.members[groupEmail] = tt.initialMembers

			// Parse time
			now, err := time.Parse(time.RFC3339, tt.currentTime)
			if err != nil {
				t.Fatalf("failed to parse time: %v", err)
			}

			// Run sync
			result, err := Sync(groupEmail, scheduleFile, mock, now)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Check results
			if !stringSlicesEqual(result.Added, tt.expectedAdded) {
				t.Errorf("added mismatch: got %v, want %v", result.Added, tt.expectedAdded)
			}
			if !stringSlicesEqual(result.Removed, tt.expectedRemoved) {
				t.Errorf("removed mismatch: got %v, want %v", result.Removed, tt.expectedRemoved)
			}

			// Verify mock was called correctly
			if !stringSlicesEqual(mock.addedCalls, tt.expectedAdded) {
				t.Errorf("AddMember calls mismatch: got %v, want %v", mock.addedCalls, tt.expectedAdded)
			}
			if !stringSlicesEqual(mock.removedCalls, tt.expectedRemoved) {
				t.Errorf("RemoveMember calls mismatch: got %v, want %v", mock.removedCalls, tt.expectedRemoved)
			}
		})
	}
}

func TestSync_SamePrimaryAndSecondary(t *testing.T) {
	// Create a schedule where primary and secondary are the same person
	scheduleContent := `2026-01-12T00:00:00Z,d@bytebase.com,d@bytebase.com`

	tmpDir := t.TempDir()
	scheduleFile := filepath.Join(tmpDir, "schedule.csv")
	if err := os.WriteFile(scheduleFile, []byte(scheduleContent), 0644); err != nil {
		t.Fatalf("failed to create schedule file: %v", err)
	}

	mock := NewMockGroupsClient()
	groupEmail := "dev-oncall@bytebase.com"
	now, _ := time.Parse(time.RFC3339, "2026-01-15T12:00:00Z")

	result, err := Sync(groupEmail, scheduleFile, mock, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only add the person once
	if len(result.Added) != 1 || result.Added[0] != "d@bytebase.com" {
		t.Errorf("expected to add d@bytebase.com once, got %v", result.Added)
	}

	// Verify member was added only once
	members, _ := mock.ListMembers(groupEmail)
	if len(members) != 1 {
		t.Errorf("expected 1 member in group, got %d", len(members))
	}
}

func TestSync_ErrorCases(t *testing.T) {
	scheduleContent := `2026-01-12T00:00:00Z,d@bytebase.com,vh@bytebase.com`
	tmpDir := t.TempDir()
	scheduleFile := filepath.Join(tmpDir, "schedule.csv")
	if err := os.WriteFile(scheduleFile, []byte(scheduleContent), 0644); err != nil {
		t.Fatalf("failed to create schedule file: %v", err)
	}

	now, _ := time.Parse(time.RFC3339, "2026-01-15T12:00:00Z")
	groupEmail := "dev-oncall@bytebase.com"

	t.Run("list members error", func(t *testing.T) {
		mock := NewMockGroupsClient()
		mock.listError = fmt.Errorf("API error")

		_, err := Sync(groupEmail, scheduleFile, mock, now)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("add member error", func(t *testing.T) {
		mock := NewMockGroupsClient()
		mock.addError = fmt.Errorf("API error")

		_, err := Sync(groupEmail, scheduleFile, mock, now)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("remove member error", func(t *testing.T) {
		mock := NewMockGroupsClient()
		mock.members[groupEmail] = []string{"old@bytebase.com"}
		mock.removeError = fmt.Errorf("API error")

		_, err := Sync(groupEmail, scheduleFile, mock, now)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("invalid schedule file", func(t *testing.T) {
		mock := NewMockGroupsClient()
		_, err := Sync(groupEmail, "/nonexistent/file.csv", mock, now)
		if err == nil {
			t.Error("expected error for invalid file")
		}
	})

	t.Run("time before first rotation", func(t *testing.T) {
		mock := NewMockGroupsClient()
		beforeTime, _ := time.Parse(time.RFC3339, "2026-01-01T00:00:00Z")
		_, err := Sync(groupEmail, scheduleFile, mock, beforeTime)
		if err == nil {
			t.Error("expected error for time before first rotation")
		}
	})
}

func TestUniqueMembers(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "no duplicates",
			input:    []string{"a@test.com", "b@test.com"},
			expected: []string{"a@test.com", "b@test.com"},
		},
		{
			name:     "with duplicates",
			input:    []string{"a@test.com", "a@test.com"},
			expected: []string{"a@test.com"},
		},
		{
			name:     "empty list",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "all same",
			input:    []string{"a@test.com", "a@test.com", "a@test.com"},
			expected: []string{"a@test.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := uniqueMembers(tt.input)
			if !stringSlicesEqual(result, tt.expected) {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDifference(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected []string
	}{
		{
			name:     "no overlap",
			a:        []string{"a@test.com", "b@test.com"},
			b:        []string{"c@test.com", "d@test.com"},
			expected: []string{"a@test.com", "b@test.com"},
		},
		{
			name:     "complete overlap",
			a:        []string{"a@test.com", "b@test.com"},
			b:        []string{"a@test.com", "b@test.com"},
			expected: []string{},
		},
		{
			name:     "partial overlap",
			a:        []string{"a@test.com", "b@test.com", "c@test.com"},
			b:        []string{"b@test.com"},
			expected: []string{"a@test.com", "c@test.com"},
		},
		{
			name:     "empty a",
			a:        []string{},
			b:        []string{"a@test.com"},
			expected: []string{},
		},
		{
			name:     "empty b",
			a:        []string{"a@test.com"},
			b:        []string{},
			expected: []string{"a@test.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := difference(tt.a, tt.b)
			if !stringSlicesEqual(result, tt.expected) {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}

// Helper function to compare string slices (order-independent for sets)
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	// For empty slices
	if len(a) == 0 {
		return true
	}

	// Create frequency maps
	aMap := make(map[string]int)
	bMap := make(map[string]int)

	for _, s := range a {
		aMap[s]++
	}
	for _, s := range b {
		bMap[s]++
	}

	// Compare maps
	for k, v := range aMap {
		if bMap[k] != v {
			return false
		}
	}
	for k, v := range bMap {
		if aMap[k] != v {
			return false
		}
	}

	return true
}
