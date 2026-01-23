package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	// Check for subcommand
	if len(os.Args) > 1 && os.Args[1] == "validate" {
		validateCommand()
		return
	}

	// Define flags
	configPath := flag.String("config", "", "Path to the oncall schedule file (required)")
	// Google Group flags
	groupEmail := flag.String("group", "", "Google Group email address to sync")
	adminUser := flag.String("admin-user", "", "Domain admin user email for impersonation (Google Group only)")
	// Slack flags
	slackGroup := flag.String("slack-group", "", "Slack User Group ID to sync")
	slackToken := flag.String("slack-token", "", "Slack API Token (can also be set via SLACK_TOKEN env var)")
	slackChannel := flag.String("slack-channel", "", "Slack Channel ID to notify on changes")

	showHelp := flag.Bool("help", false, "Show usage information")

	flag.Parse()

	// Show help
	if *showHelp {
		printUsage()
		os.Exit(0)
	}

	// Validate required flags
	if *configPath == "" {
		fmt.Fprintf(os.Stderr, "Error: --config flag is required\n\n")
		printUsage()
		os.Exit(1)
	}

	syncPerformed := false
	anyChanges := false
	var currentRotation *Rotation

	// Initialize Slack Client if needed (for sync or notifications)
	var slackClient *SlackClient
	token := *slackToken
	if token == "" {
		token = os.Getenv("SLACK_TOKEN")
	}

	if token != "" {
		slackClient = NewSlackClient(token)
	} else if *slackGroup != "" || *slackChannel != "" {
		fmt.Fprintf(os.Stderr, "Error: Slack token is required via --slack-token or SLACK_TOKEN env var for Slack sync or notifications\n")
		os.Exit(1)
	}

	// Google Group Sync
	if *groupEmail != "" {
		syncPerformed = true
		fmt.Println("--- Starting Google Group Sync ---")

		if *adminUser == "" {
			fmt.Fprintf(os.Stderr, "Error: --admin-user flag is required for Google Group sync\n\n")
			printUsage()
			os.Exit(1)
		}

		// Get credentials from environment
		credentialsJSON := os.Getenv("GOOGLE_CREDENTIALS")
		if credentialsJSON == "" {
			fmt.Fprintf(os.Stderr, "Error: GOOGLE_CREDENTIALS environment variable not set\n")
			fmt.Fprintf(os.Stderr, "Please set it to your Google service account JSON key content.\n")
			os.Exit(1)
		}

		// Validate credentials format
		if err := ValidateCredentials([]byte(credentialsJSON)); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Create Google Groups client
		ctx := context.Background()
		googleClient, err := NewGoogleGroupsClient(ctx, []byte(credentialsJSON), *adminUser)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: authentication failed - %v\n", err)
			fmt.Fprintf(os.Stderr, "Please check that your service account has domain-wide delegation enabled.\n")
			os.Exit(1)
		}

		changed, rot, err := runSync(*configPath, *groupEmail, googleClient)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Google Group sync failed - %v\n", err)
			os.Exit(1)
		}
		if changed {
			anyChanges = true
		}
		currentRotation = rot

		fmt.Printf("--- Google Group Sync Completed ---\n\n")
	}

	// Slack Sync
	if *slackGroup != "" {
		syncPerformed = true
		fmt.Println("--- Starting Slack Sync ---")

		// slackClient is already initialized above

		changed, rot, err := runSync(*configPath, *slackGroup, slackClient)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Slack sync failed - %v\n", err)
			os.Exit(1)
		}
		if changed {
			anyChanges = true
		}
		currentRotation = rot // Either sync source provides valid rotation
		fmt.Printf("--- Slack Sync Completed ---\n\n")
	}

	if !syncPerformed {
		fmt.Fprintf(os.Stderr, "Error: No sync target specified. Provide --group (Google Groups) or --slack-group (Slack) or both.\n\n")
		printUsage()
		os.Exit(1)
	}

	// Send notification if configured and changes were made
	if anyChanges && slackClient != nil && *slackChannel != "" && currentRotation != nil {
		fmt.Printf("--- Sending Notification ---\n")
		msg := fmt.Sprintf("On-call rotation update.\n\nCurrent on-call:\n• Primary: %s\n• Secondary: %s",
			currentRotation.Primary, currentRotation.Secondary)

		fmt.Printf("Sending notification to channel %s...\n", *slackChannel)
		if err := slackClient.PostMessage(*slackChannel, msg); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to send Slack notification: %v\n", err)
		}
	}
}

func runSync(configPath string, groupEmail string, client GroupsClient) (bool, *Rotation, error) {
	fmt.Printf("Reading schedule from: %s\n", configPath)

	// Get current time
	now := time.Now()

	// Parse schedule and find current rotation
	rotations, err := ParseSchedule(configPath)
	if err != nil {
		return false, nil, err
	}

	currentRotation, err := FindCurrentRotation(rotations, now)
	if err != nil {
		return false, nil, err
	}

	fmt.Printf("Current rotation: %s - Primary: %s, Secondary: %s\n",
		currentRotation.StartTime.Format("2006-01-02"),
		currentRotation.Primary,
		currentRotation.Secondary)

	// Sync group membership
	fmt.Printf("Syncing group: %s\n", groupEmail)

	result, err := Sync(groupEmail, configPath, client, now)
	if err != nil {
		return false, nil, err
	}

	// Report results
	if len(result.Removed) == 0 && len(result.Added) == 0 {
		fmt.Println("  No changes needed.")
		return false, currentRotation, nil
	} else {
		for _, member := range result.Removed {
			fmt.Printf("  Removed: %s\n", member)
		}
		for _, member := range result.Added {
			fmt.Printf("  Added: %s\n", member)
		}
		return true, currentRotation, nil
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `synconcall - Sync oncall rotation to Google Group or Slack User Group

Usage:
  synconcall --config=<path> [--group=<email> --admin-user=<email>] [--slack-group=<id> --slack-token=<token>] [--slack-channel=<channel_id>]

Required Flags:
  --config string
        Path to the oncall schedule file (CSV format)

Google Group Flags:
  --group string
        Google Group email address to sync
  --admin-user string
        Domain admin email for service account impersonation
  (Requires GOOGLE_CREDENTIALS environment variable)

Slack Flags:
  --slack-group string
        Slack User Group ID to sync
  --slack-token string
        Slack API Token (can also be set via SLACK_TOKEN env var)
  --slack-channel string
        Slack Channel ID to notify on changes

Optional Flags:
  --help
        Show this usage information

Examples:
  # Google Groups (GitHub Actions)
  GOOGLE_CREDENTIALS="${{ secrets.GOOGLE_SERVICE_ACCOUNT }}" \
    synconcall --config=dev.oncall --group=dev-oncall@bytebase.com --admin-user=admin@bytebase.com

  # Slack (GitHub Actions)
  SLACK_TOKEN="${{ secrets.SLACK_TOKEN }}" \
    synconcall --config=dev.oncall --slack-group=S0123456789

Schedule File Format:
  CSV format with 3 columns: timestamp,primary_email,secondary_email
  Timestamps in RFC3339 format (ISO 8601)

  Example:
    2026-01-12T00:00:00Z,d@bytebase.com,vh@bytebase.com
    2026-02-09T00:00:00Z,vh@bytebase.com,xz@bytebase.com
`)
}

func validateCommand() {
	// Define flags for validate subcommand
	validateFlags := flag.NewFlagSet("validate", flag.ExitOnError)
	configPath := validateFlags.String("config", "", "Path to the oncall schedule file (required)")

	validateFlags.Parse(os.Args[2:])

	if *configPath == "" {
		fmt.Fprintf(os.Stderr, "Error: --config flag is required\n")
		fmt.Fprintf(os.Stderr, "Usage: synconcall validate --config=<path>\n")
		os.Exit(1)
	}

	// Parse and validate schedule
	rotations, err := ParseSchedule(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ Schedule validation failed: %v\n", err)
		os.Exit(1)
	}

	// Find current rotation (validates that schedule has valid time ranges)
	now := time.Now()
	currentRotation, err := FindCurrentRotation(rotations, now)
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ Schedule validation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Schedule is valid\n")
	fmt.Printf("  Total rotations: %d\n", len(rotations))
	fmt.Printf("  Current rotation: %s - Primary: %s, Secondary: %s\n",
		currentRotation.StartTime.Format("2006-01-02"),
		currentRotation.Primary,
		currentRotation.Secondary)
}
