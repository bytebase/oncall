package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

const githubAPIBase = "https://api.github.com"

// GitHubTeamClient implements GroupsClient for a GitHub organization team.
//
// The rest of the tool works in terms of oncall emails, but GitHub teams are
// keyed by username. This client holds an email <-> username mapping (loaded
// from a CSV file) and translates between the two, mirroring how SlackClient
// translates email <-> Slack user ID.
type GitHubTeamClient struct {
	org         string
	token       string
	http        *http.Client
	emailToUser map[string]string
	userToEmail map[string]string
}

// NewGitHubTeamClient creates a client for the given org using the supplied
// token and email->username mapping.
func NewGitHubTeamClient(org, token string, emailToUser map[string]string) *GitHubTeamClient {
	userToEmail := make(map[string]string, len(emailToUser))
	for email, user := range emailToUser {
		userToEmail[strings.ToLower(user)] = email
	}
	return &GitHubTeamClient{
		org:         org,
		token:       token,
		http:        &http.Client{},
		emailToUser: emailToUser,
		userToEmail: userToEmail,
	}
}

// LoadUserMapping reads an email,github_username CSV file into a map.
// Blank lines and lines starting with '#' are ignored.
func LoadUserMapping(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open user mapping %s: %w", path, err)
	}
	defer file.Close()

	mapping := make(map[string]string)
	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid mapping on line %d: expected email,github_username", lineNo)
		}
		email := strings.TrimSpace(parts[0])
		user := strings.TrimSpace(parts[1])
		if email == "" || user == "" {
			return nil, fmt.Errorf("invalid mapping on line %d: empty field", lineNo)
		}
		mapping[email] = user
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read user mapping %s: %w", path, err)
	}
	return mapping, nil
}

// usernameFor resolves an oncall email to a GitHub username. If the value is
// already a username (returned by ListMembers for an unmapped member), it is
// returned unchanged.
func (c *GitHubTeamClient) usernameFor(member string) string {
	if user, ok := c.emailToUser[member]; ok {
		return user
	}
	return member
}

// ListMembers returns the team members keyed by email when known, falling back
// to the raw GitHub username for members not present in the mapping (so the
// team is fully reconciled to the current rotation).
func (c *GitHubTeamClient) ListMembers(teamSlug string) ([]string, error) {
	var members []string
	page := 1
	for {
		path := fmt.Sprintf("/orgs/%s/teams/%s/members?per_page=100&page=%d", c.org, teamSlug, page)
		var batch []struct {
			Login string `json:"login"`
		}
		if err := c.do(http.MethodGet, path, &batch); err != nil {
			return nil, fmt.Errorf("failed to list team members: %w", err)
		}
		if len(batch) == 0 {
			break
		}
		for _, m := range batch {
			if email, ok := c.userToEmail[strings.ToLower(m.Login)]; ok {
				members = append(members, email)
			} else {
				members = append(members, m.Login)
			}
		}
		page++
	}
	return members, nil
}

// AddMember adds (or affirms) a member's team membership. Sync only ever asks
// to add desired-state oncall emails, so the email must be present in the
// mapping.
func (c *GitHubTeamClient) AddMember(teamSlug, member string) error {
	username, ok := c.emailToUser[member]
	if !ok {
		return fmt.Errorf("no github username mapped for %s", member)
	}
	path := fmt.Sprintf("/orgs/%s/teams/%s/memberships/%s", c.org, teamSlug, username)
	body := strings.NewReader(`{"role":"member"}`)
	if err := c.doBody(http.MethodPut, path, body, nil); err != nil {
		return fmt.Errorf("failed to add member %s: %w", username, err)
	}
	return nil
}

// RemoveMember removes a member from the team.
func (c *GitHubTeamClient) RemoveMember(teamSlug, member string) error {
	username := c.usernameFor(member)
	path := fmt.Sprintf("/orgs/%s/teams/%s/memberships/%s", c.org, teamSlug, username)
	if err := c.doBody(http.MethodDelete, path, nil, nil); err != nil {
		return fmt.Errorf("failed to remove member %s: %w", username, err)
	}
	return nil
}

func (c *GitHubTeamClient) do(method, path string, out any) error {
	return c.doBody(method, path, nil, out)
}

func (c *GitHubTeamClient) doBody(method, path string, body *strings.Reader, out any) error {
	var reqBody *strings.Reader
	if body != nil {
		reqBody = body
	} else {
		reqBody = strings.NewReader("")
	}
	req, err := http.NewRequest(method, githubAPIBase+path, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr struct {
			Message string `json:"message"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		return fmt.Errorf("github API %s %s: %d %s", method, path, resp.StatusCode, apiErr.Message)
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}
	return nil
}
