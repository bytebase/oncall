package main

import (
	"context"
	"encoding/json"
	"fmt"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/option"
)

// GroupsClient defines the interface for Google Groups operations
type GroupsClient interface {
	ListMembers(groupEmail string) ([]string, error)
	AddMember(groupEmail, memberEmail string) error
	RemoveMember(groupEmail, memberEmail string) error
}

// GoogleGroupsClient implements GroupsClient using Google Admin SDK
type GoogleGroupsClient struct {
	service *admin.Service
	ctx     context.Context
}

// NewGoogleGroupsClient creates a new Google Groups client using service account credentials
// subject is the email of a domain admin user that the service account will impersonate
func NewGoogleGroupsClient(ctx context.Context, credentialsJSON []byte, subject string) (*GoogleGroupsClient, error) {
	config, err := google.JWTConfigFromJSON(credentialsJSON,
		"https://www.googleapis.com/auth/admin.directory.group",
		"https://www.googleapis.com/auth/admin.directory.group.member")
	if err != nil {
		return nil, fmt.Errorf("failed to parse credentials: %w", err)
	}

	// Set the subject for domain-wide delegation
	config.Subject = subject

	service, err := admin.NewService(ctx, option.WithHTTPClient(config.Client(ctx)))
	if err != nil {
		return nil, fmt.Errorf("failed to create admin service: %w", err)
	}

	return &GoogleGroupsClient{
		service: service,
		ctx:     ctx,
	}, nil
}

// ListMembers retrieves all member email addresses from a Google Group
func (c *GoogleGroupsClient) ListMembers(groupEmail string) ([]string, error) {
	var members []string
	pageToken := ""

	for {
		call := c.service.Members.List(groupEmail)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		resp, err := call.Do()
		if err != nil {
			return nil, fmt.Errorf("failed to list members: %w", err)
		}

		for _, member := range resp.Members {
			members = append(members, member.Email)
		}

		pageToken = resp.NextPageToken
		if pageToken == "" {
			break
		}
	}

	return members, nil
}

// AddMember adds a member to a Google Group with MEMBER role
func (c *GoogleGroupsClient) AddMember(groupEmail, memberEmail string) error {
	member := &admin.Member{
		Email: memberEmail,
		Role:  "MEMBER",
	}

	_, err := c.service.Members.Insert(groupEmail, member).Do()
	if err != nil {
		// Check if member already exists (idempotent)
		if isAlreadyExistsError(err) {
			return nil
		}
		return fmt.Errorf("failed to add member %s: %w", memberEmail, err)
	}

	return nil
}

// RemoveMember removes a member from a Google Group
func (c *GoogleGroupsClient) RemoveMember(groupEmail, memberEmail string) error {
	err := c.service.Members.Delete(groupEmail, memberEmail).Do()
	if err != nil {
		// Check if member doesn't exist (idempotent)
		if isNotFoundError(err) {
			return nil
		}
		return fmt.Errorf("failed to remove member %s: %w", memberEmail, err)
	}

	return nil
}

// isAlreadyExistsError checks if the error indicates the member already exists
func isAlreadyExistsError(err error) bool {
	// Google API returns specific error messages for duplicate members
	// This is a basic check - in production you'd want more robust error handling
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return contains(errMsg, "already exists") || contains(errMsg, "duplicate") || contains(errMsg, "Member already exists")
}

// isNotFoundError checks if the error indicates the member was not found
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return contains(errMsg, "not found") || contains(errMsg, "404") || contains(errMsg, "Resource Not Found")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findInString(s, substr)
}

func findInString(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ValidateCredentials validates that the credentials JSON is properly formatted
func ValidateCredentials(credentialsJSON []byte) error {
	var creds map[string]interface{}
	if err := json.Unmarshal(credentialsJSON, &creds); err != nil {
		return fmt.Errorf("invalid service account credentials format: %w", err)
	}
	return nil
}
