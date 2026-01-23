package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	// Define flags
	configPath := flag.String("config", "", "Path to the oncall schedule file (required)")
	groupEmail := flag.String("group", "", "Google Group email address to sync (required)")
	adminUser := flag.String("admin-user", "", "Domain admin user email for impersonation (required)")
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

	if *groupEmail == "" {
		fmt.Fprintf(os.Stderr, "Error: --group flag is required\n\n")
		printUsage()
		os.Exit(1)
	}

	if *adminUser == "" {
		fmt.Fprintf(os.Stderr, "Error: --admin-user flag is required\n\n")
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
	client, err := NewGoogleGroupsClient(ctx, []byte(credentialsJSON), *adminUser)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: authentication failed - %v\n", err)
		fmt.Fprintf(os.Stderr, "Please check that your service account has domain-wide delegation enabled.\n")
		os.Exit(1)
	}

	// Run sync
	if err := runSync(*configPath, *groupEmail, client); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runSync(configPath string, groupEmail string, client GroupsClient) error {
	fmt.Printf("Reading schedule from: %s\n", configPath)

	// Get current time
	now := time.Now()

	// Parse schedule and find current rotation
	rotations, err := ParseSchedule(configPath)
	if err != nil {
		return err
	}

	currentRotation, err := FindCurrentRotation(rotations, now)
	if err != nil {
		return err
	}

	fmt.Printf("Current rotation: %s - Primary: %s, Secondary: %s\n",
		currentRotation.StartTime.Format("2006-01-02"),
		currentRotation.Primary,
		currentRotation.Secondary)

	// Sync group membership
	fmt.Printf("Syncing group: %s\n", groupEmail)

	result, err := Sync(groupEmail, configPath, client, now)
	if err != nil {
		return err
	}

	// Report results
	if len(result.Removed) == 0 && len(result.Added) == 0 {
		fmt.Println("  No changes needed.")
	} else {
		for _, member := range result.Removed {
			fmt.Printf("  Removed: %s\n", member)
		}
		for _, member := range result.Added {
			fmt.Printf("  Added: %s\n", member)
		}
	}

	fmt.Println("Sync completed successfully.")
	return nil
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `synconcall - Sync oncall rotation to Google Group

Usage:
  synconcall --config=<path> --group=<email> --admin-user=<email>

Required Flags:
  --config string
        Path to the oncall schedule file (CSV format)
  --group string
        Google Group email address to sync
  --admin-user string
        Domain admin email for service account impersonation

Optional Flags:
  --help
        Show this usage information

Environment Variables:
  GOOGLE_CREDENTIALS (required)
        Google service account JSON key content

Examples:
  # GitHub Actions
  GOOGLE_CREDENTIALS="${{ secrets.GOOGLE_SERVICE_ACCOUNT }}" \
    synconcall --config=dev.oncall --group=dev-oncall@bytebase.com --admin-user=admin@bytebase.com

  # Local testing
  GOOGLE_CREDENTIALS="$(cat service-account.json)" \
    synconcall --config=dev.oncall --group=dev-oncall@bytebase.com --admin-user=admin@bytebase.com

Schedule File Format:
  CSV format with 3 columns: timestamp,primary_email,secondary_email
  Timestamps in RFC3339 format (ISO 8601)

  Example:
    2026-01-12T00:00:00Z,d@bytebase.com,vh@bytebase.com
    2026-02-09T00:00:00Z,vh@bytebase.com,xz@bytebase.com
`)
}
