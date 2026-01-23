package main

import (
	"fmt"
	"time"
)

// SyncResult contains the results of a sync operation
type SyncResult struct {
	Added   []string
	Removed []string
}

// Sync reconciles the Google Group membership with the current oncall rotation
func Sync(groupEmail string, configPath string, client GroupsClient, now time.Time) (*SyncResult, error) {
	// Parse schedule
	rotations, err := ParseSchedule(configPath)
	if err != nil {
		return nil, err
	}

	// Find current rotation
	currentRotation, err := FindCurrentRotation(rotations, now)
	if err != nil {
		return nil, err
	}

	// Get desired members (deduplicated in case primary == secondary)
	desiredMembers := uniqueMembers([]string{currentRotation.Primary, currentRotation.Secondary})

	// Get current group members
	currentMembers, err := client.ListMembers(groupEmail)
	if err != nil {
		return nil, fmt.Errorf("failed to list group members: %w", err)
	}

	// Calculate diff
	toAdd := difference(desiredMembers, currentMembers)
	toRemove := difference(currentMembers, desiredMembers)

	result := &SyncResult{
		Added:   make([]string, 0),
		Removed: make([]string, 0),
	}

	// Remove members first
	for _, member := range toRemove {
		if err := client.RemoveMember(groupEmail, member); err != nil {
			return nil, err
		}
		result.Removed = append(result.Removed, member)
	}

	// Add new members
	for _, member := range toAdd {
		if err := client.AddMember(groupEmail, member); err != nil {
			return nil, err
		}
		result.Added = append(result.Added, member)
	}

	return result, nil
}

// uniqueMembers returns a deduplicated list of members
func uniqueMembers(members []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(members))

	for _, member := range members {
		if !seen[member] {
			seen[member] = true
			result = append(result, member)
		}
	}

	return result
}

// difference returns elements in a that are not in b
func difference(a, b []string) []string {
	bSet := make(map[string]bool)
	for _, item := range b {
		bSet[item] = true
	}

	result := make([]string, 0)
	for _, item := range a {
		if !bSet[item] {
			result = append(result, item)
		}
	}

	return result
}
