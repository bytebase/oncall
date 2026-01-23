# synconcall

A Go CLI tool that synchronizes the current oncall rotation to a Google Group based on a schedule file.

## Features

- Reads oncall rotation schedule from CSV file
- Determines current oncall based on current time
- Syncs Google Group membership to match current rotation (declarative set)
- Comprehensive error handling and validation
- Fully tested with unit tests

## Prerequisites

- Go 1.25.6 or later
- Google Workspace admin access
- Service account with domain-wide delegation
- Required Google API scopes:
  - `https://www.googleapis.com/auth/admin.directory.group`
  - `https://www.googleapis.com/auth/admin.directory.group.member`

## Installation

```bash
go build -o synconcall
```

## Usage

### Command Line

```bash
synconcall --config=<path> --group=<email>
```

**Required Flags:**
- `--config`: Path to the oncall schedule file (CSV format)
- `--group`: Google Group email address to sync

**Environment Variables:**
- `GOOGLE_CREDENTIALS` (required): Google service account JSON key content

### Examples

#### GitHub Actions

```yaml
- name: Sync oncall rotation
  env:
    GOOGLE_CREDENTIALS: ${{ secrets.GOOGLE_SERVICE_ACCOUNT }}
  run: |
    ./synconcall --config=dev.oncall --group=dev-oncall@bytebase.com
```

#### Local Testing

```bash
GOOGLE_CREDENTIALS="$(cat service-account.json)" \
  ./synconcall --config=dev.oncall --group=dev-oncall@bytebase.com
```

## Schedule File Format

CSV format with 3 columns: `timestamp,primary_email,secondary_email`

- Timestamps in RFC3339 format (ISO 8601)
- Each line represents a rotation period starting at that timestamp
- Rotations must be in ascending chronological order

**Example:**

```csv
2026-01-12T00:00:00Z,d@bytebase.com,vh@bytebase.com
2026-02-09T00:00:00Z,vh@bytebase.com,xz@bytebase.com
2026-03-09T00:00:00Z,xz@bytebase.com,zp@bytebase.com
```

## How It Works

1. Parses the schedule file and validates format
2. Finds the current rotation based on current time
3. Authenticates with Google using service account credentials
4. Fetches current Google Group membership
5. Calculates diff between desired and current membership
6. Removes old members and adds current oncall people
7. Reports what changed

The sync is **declarative** - the group membership will exactly match the current rotation.

## Error Handling

- **Schedule file errors**: Invalid format, missing file, bad timestamps, etc.
- **Google API errors**: Authentication failures, permission issues, network errors
- **Edge cases**: Time before first rotation, same person as primary and secondary, etc.

All errors are reported to stderr with clear, actionable messages. The program exits with:
- `0` on success
- `1` on any error

## Testing

Run all tests:

```bash
go test -v
```

Run specific tests:

```bash
go test -v -run TestParseSchedule
go test -v -run TestFindCurrentRotation
go test -v -run TestSync
```

## Output Examples

### Success with changes

```
Reading schedule from: dev.oncall
Current rotation: 2026-01-12 - Primary: d@bytebase.com, Secondary: vh@bytebase.com
Syncing group: dev-oncall@bytebase.com
  Removed: old-person@bytebase.com
  Added: d@bytebase.com
  Added: vh@bytebase.com
Sync completed successfully.
```

### Success with no changes

```
Reading schedule from: dev.oncall
Current rotation: 2026-01-12 - Primary: d@bytebase.com, Secondary: vh@bytebase.com
Syncing group: dev-oncall@bytebase.com
  No changes needed.
Sync completed successfully.
```

### Error example

```
Error: failed to parse schedule: invalid timestamp format on line 3
```

## Service Account Setup

1. Create a service account in Google Cloud Console
2. Download the JSON key file
3. Enable domain-wide delegation for the service account
4. Grant the following OAuth scopes in Google Workspace Admin:
   - `https://www.googleapis.com/auth/admin.directory.group`
   - `https://www.googleapis.com/auth/admin.directory.group.member`
5. Store the JSON key content in GitHub Secrets as `GOOGLE_SERVICE_ACCOUNT`

## Project Structure

```
synconcall/
├── main.go           # CLI entry point and orchestration
├── schedule.go       # Schedule parsing and rotation finding
├── schedule_test.go  # Tests for schedule logic
├── groups.go         # Google Groups API client
├── sync.go           # Sync logic (diff calculation and reconciliation)
├── sync_test.go      # Tests for sync logic
├── go.mod            # Go module definition
├── go.sum            # Go module checksums
└── README.md         # This file
```
