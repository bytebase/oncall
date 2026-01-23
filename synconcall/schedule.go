package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

// Rotation represents an oncall rotation period
type Rotation struct {
	StartTime time.Time
	Primary   string
	Secondary string
}

// ParseSchedule reads and parses the schedule file
func ParseSchedule(filepath string) ([]Rotation, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open schedule file: %w", err)
	}
	defer file.Close()

	var rotations []Rotation
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines
		if line == "" {
			continue
		}

		parts := strings.Split(line, ",")
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid schedule format on line %d: expected 3 fields, got %d", lineNum, len(parts))
		}

		timestamp := strings.TrimSpace(parts[0])
		primary := strings.TrimSpace(parts[1])
		secondary := strings.TrimSpace(parts[2])

		// Parse timestamp
		startTime, err := time.Parse(time.RFC3339, timestamp)
		if err != nil {
			return nil, fmt.Errorf("invalid timestamp format on line %d: %w", lineNum, err)
		}

		rotation := Rotation{
			StartTime: startTime,
			Primary:   primary,
			Secondary: secondary,
		}

		// Validate rotation
		if err := ValidateRotation(rotation); err != nil {
			return nil, fmt.Errorf("invalid rotation on line %d: %w", lineNum, err)
		}

		// Check ascending order
		if len(rotations) > 0 && !startTime.After(rotations[len(rotations)-1].StartTime) {
			return nil, fmt.Errorf("timestamps not in ascending order on line %d", lineNum)
		}

		rotations = append(rotations, rotation)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading schedule file: %w", err)
	}

	if len(rotations) == 0 {
		return nil, fmt.Errorf("schedule file is empty")
	}

	return rotations, nil
}

// ValidateRotation validates a single rotation entry
func ValidateRotation(r Rotation) error {
	if !isValidEmail(r.Primary) {
		return fmt.Errorf("invalid primary email: %s", r.Primary)
	}
	if !isValidEmail(r.Secondary) {
		return fmt.Errorf("invalid secondary email: %s", r.Secondary)
	}
	return nil
}

// isValidEmail performs basic email validation
func isValidEmail(email string) bool {
	if email == "" {
		return false
	}
	// Basic check: contains @ and at least one dot after @
	atIndex := strings.Index(email, "@")
	if atIndex == -1 || atIndex == 0 || atIndex == len(email)-1 {
		return false
	}
	dotIndex := strings.LastIndex(email, ".")
	return dotIndex > atIndex && dotIndex < len(email)-1
}

// FindCurrentRotation finds the rotation active at the given time
func FindCurrentRotation(rotations []Rotation, now time.Time) (*Rotation, error) {
	if len(rotations) == 0 {
		return nil, fmt.Errorf("no rotations available")
	}

	// Find the latest rotation where StartTime <= now
	var current *Rotation
	for i := range rotations {
		if rotations[i].StartTime.After(now) {
			break
		}
		current = &rotations[i]
	}

	if current == nil {
		return nil, fmt.Errorf("no active rotation (current time is before first rotation start)")
	}

	return current, nil
}
