# Oncall Rotation System

Automated oncall rotation management system that syncs the current oncall schedule to a Google Group.

## Overview

This repository contains:
- **`dev.oncall`**: Oncall rotation schedule (CSV format)
- **`synconcall/`**: Go CLI tool that syncs current rotation to Google Group
- **GitHub Actions**: Automated daily sync workflow

## Quick Start

### Schedule File Format

Edit `dev.oncall` to define your rotation schedule:

```csv
2026-01-12T00:00:00Z,d@bytebase.com,vh@bytebase.com
2026-02-09T00:00:00Z,vh@bytebase.com,xz@bytebase.com
2026-03-09T00:00:00Z,xz@bytebase.com,zp@bytebase.com
```

Each line: `start_time,primary_email,secondary_email`
- Timestamps in RFC3339 format (ISO 8601)
- Rotations in chronological order
- Each rotation period starts at the specified timestamp

### How It Works

1. GitHub Actions runs daily (configurable)
2. Reads `dev.oncall` to determine current rotation
3. Syncs Google Group membership to match current primary + secondary
4. Old oncall members are removed, current ones are added

The sync is **declarative** - the group always reflects exactly who is currently oncall.

## Setup

**Service Account:** `dev-tools@bytebase-dev.iam.gserviceaccount.com`

### 1. Create Service Account

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a service account
3. Download JSON key file
4. Enable domain-wide delegation

### 2. Grant API Permissions

In Google Workspace Admin Console:
1. Go to Security > API Controls > Domain-wide Delegation
2. Add the service account client ID
3. Grant OAuth scopes (comma-separated):
   - `https://www.googleapis.com/auth/admin.directory.group,https://www.googleapis.com/auth/admin.directory.group.member`

**Note:** The service account uses domain-wide delegation to impersonate a domain admin user (`d@bytebase.com`) to access the Admin SDK.

### 3. Configure GitHub Secrets

Add repository secret:
- Name: `GOOGLE_SERVICE_ACCOUNT`
- Value: Contents of the service account JSON key file

### 4. Customize Workflow

Edit `.github/workflows/sync-oncall.yml`:
- Change cron schedule (default: daily at midnight UTC)
- Update group email if different from `dev-oncall@bytebase.com`

## Manual Sync

Build and run locally:

```bash
cd synconcall
go build -o synconcall

GOOGLE_CREDENTIALS="$(cat /path/to/service-account.json)" \
  ./synconcall --config=../dev.oncall --group=dev-oncall@bytebase.com --admin-user=d@bytebase.com
```

## Updating the Schedule

1. Edit `dev.oncall` to add new rotation periods
2. Commit and push changes
3. Next scheduled run will pick up the changes
4. Or trigger manually: Actions tab → "Sync Oncall to Google Group" → Run workflow

## Project Structure

```
oncall/
├── dev.oncall                     # Rotation schedule
├── synconcall/                    # Sync tool
│   ├── main.go                   # CLI entry point
│   ├── schedule.go               # Schedule parsing
│   ├── groups.go                 # Google API client
│   ├── sync.go                   # Sync logic
│   ├── *_test.go                 # Comprehensive tests
│   └── README.md                 # Tool documentation
├── .github/workflows/
│   └── sync-oncall.yml           # Automated sync workflow
└── docs/plans/
    └── 2026-01-23-oncall-sync-design.md  # Design document
```

## Testing

Run tests:
```bash
cd synconcall
go test -v
```

## Troubleshooting

### Sync fails with authentication error
- Check service account has domain-wide delegation enabled
- Verify OAuth scopes are granted correctly
- Ensure JSON key is valid in GitHub secrets

### Members not updating
- Check schedule file format (timestamps, emails)
- Verify current time falls within a rotation period
- Check GitHub Actions logs for errors

### Wrong people in group
- Verify `dev.oncall` has correct rotation schedule
- Check timestamps are in chronological order
- Ensure timestamps use RFC3339 format with timezone

## Design Documentation

See [docs/plans/2026-01-23-oncall-sync-design.md](docs/plans/2026-01-23-oncall-sync-design.md) for detailed design decisions and architecture.