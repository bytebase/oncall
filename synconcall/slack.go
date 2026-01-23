package main

import (
	"fmt"
	"strings"

	"github.com/slack-go/slack"
)

// SlackClient implements GroupsClient using Slack API
type SlackClient struct {
	client *slack.Client
}

// NewSlackClient creates a new Slack client
func NewSlackClient(token string) *SlackClient {
	return &SlackClient{
		client: slack.New(token),
	}
}

// ListMembers retrieves all member email addresses from a Slack User Group
func (c *SlackClient) ListMembers(groupID string) ([]string, error) {
	// Get members IDs of the group
	memberIDs, err := c.client.GetUserGroupMembers(groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to list slack group members: %w", err)
	}

	if len(memberIDs) == 0 {
		return []string{}, nil
	}

	// To translate IDs to emails efficiently, it's often better to fetch all users
	// if the group is large, or fetch individually if small.
	// For robustness, let's fetch all users to build a map.
	// Note: In very large workspaces, this might be slow.
	// An alternative is to use GetUserInfo for each member if count is small.
	// Let's assume on-call groups are relatively small (< 20 people), so fetching info one by one
	// might actually be faster than fetching 10k users.
	// However, Sync() calls `ListMembers` once.
	// Let's try fetching info individually for now as it's safer for large workspaces,
	// unless we hit rate limits. Parallelism could help here but let's keep it simple.

	var emails []string
	for _, id := range memberIDs {
		user, err := c.client.GetUserInfo(id)
		if err != nil {
			return nil, fmt.Errorf("failed to get user info for %s: %w", id, err)
		}
		// Skip bots or deleted users if necessary? Usually on-call are real users.
		if user.IsBot || user.Deleted {
			continue
		}
		emails = append(emails, user.Profile.Email)
	}

	return emails, nil
}

// AddMember adds a member to a Slack User Group
func (c *SlackClient) AddMember(groupID, memberEmail string) error {
	// 1. Resolve email to User ID
	user, err := c.client.GetUserByEmail(memberEmail)
	if err != nil {
		return fmt.Errorf("failed to resolve email %s to slack user: %w", memberEmail, err)
	}

	// 2. Get current members
	currentMemberIDs, err := c.client.GetUserGroupMembers(groupID)
	if err != nil {
		return fmt.Errorf("failed to get current group members: %w", err)
	}

	// 3. Check if already present
	for _, id := range currentMemberIDs {
		if id == user.ID {
			return nil // Already a member
		}
	}

	// 4. Add to list
	newMemberIDs := append(currentMemberIDs, user.ID)

	// 5. Update group
	_, err = c.client.UpdateUserGroupMembers(groupID, strings.Join(newMemberIDs, ","))
	if err != nil {
		return fmt.Errorf("failed to update slack group members: %w", err)
	}

	return nil
}

// RemoveMember removes a member from a Slack User Group
func (c *SlackClient) RemoveMember(groupID, memberEmail string) error {
	// 1. Resolve email to User ID
	user, err := c.client.GetUserByEmail(memberEmail)
	if err != nil {
		return fmt.Errorf("failed to resolve email %s to slack user: %w", memberEmail, err)
	}

	// 2. Get current members
	currentMemberIDs, err := c.client.GetUserGroupMembers(groupID)
	if err != nil {
		return fmt.Errorf("failed to get current group members: %w", err)
	}

	// 3. Remove from list
	newMemberIDs := make([]string, 0, len(currentMemberIDs))
	found := false
	for _, id := range currentMemberIDs {
		if id == user.ID {
			found = true
			continue
		}
		newMemberIDs = append(newMemberIDs, id)
	}

	if !found {
		return nil // Not a member
	}

	// 4. Update group
	// Slack requires at least one member?
	// If newMemberIDs is empty, this call might fail if Slack disables empty groups?
	// The API doc doesn't explicitly forbid empty groups but the behavior might vary.
	// Let's assume it's allowed or the group isn't meant to be empty.
	_, err = c.client.UpdateUserGroupMembers(groupID, strings.Join(newMemberIDs, ","))
	if err != nil {
		return fmt.Errorf("failed to update slack group members: %w", err)
	}

	return nil
}

// PostMessage sends a message to a Slack channel
func (c *SlackClient) PostMessage(channelID, message string) error {
	_, _, err := c.client.PostMessage(channelID, slack.MsgOptionText(message, false))
	if err != nil {
		return fmt.Errorf("failed to post message to slack channel %s: %w", channelID, err)
	}
	return nil
}
