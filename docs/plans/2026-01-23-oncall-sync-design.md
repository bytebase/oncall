# Oncall Sync Tool Design

## Overview

A Go CLI tool that synchronizes the current oncall rotation to a Google Group (dev-oncall@bytebase.com) based on a schedule file. The tool determines who is currently oncall and ensures the Google Group membership exactly matches (declarative set).

## Architecture

The program is a stateless CLI tool that:

1. Reads the rotation schedule from a config file
2. Determines the current rotation based on current time
3. Authenticates with Google Groups API using a service account
4. Fetches current group membership
5. Reconciles membership to match the schedule (removes old, adds current)
6. Reports what changed

### Project Structure

```
oncall/
├── dev.oncall                    # Rotation schedule (top level)
└── synconcall/                   # Go program directory
    ├── main.go                   # CLI entry point, flags, orchestration
    ├── schedule.go               # Parse dev.oncall, find current rotation
    ├── schedule_test.go          # Tests for schedule parsing and rotation logic
    ├── groups.go                 # Google Groups API client wrapper
    ├── sync.go                   # Sync logic (calculate diff, apply changes)
    ├── sync_test.go              # Tests for sync logic
    └── go.mod
```

## Schedule File Format

### Format Specification

- CSV format: `timestamp,primary_email,secondary_email`
- Timestamp in RFC3339 format (ISO 8601)
- Each line represents a rotation period starting at that timestamp
- Rotations ordered chronologically
- Current rotation = latest timestamp where timestamp <= current time

### Example

```
2026-01-12T00:00:00Z,d@bytebase.com,vh@bytebase.com
2026-02-09T00:00:00Z,vh@bytebase.com,xz@bytebase.com
2026-03-09T00:00:00Z,xz@bytebase.com,zp@bytebase.com
```

### Validation Rules

1. Each line has exactly 3 comma-separated fields
2. Timestamp parses as valid RFC3339
3. Timestamps are in ascending order
4. Emails look valid (basic format check: contains @ and .)
5. File has at least one rotation
6. No duplicate emails in same rotation

### Code Structure

```go
type Rotation struct {
    StartTime time.Time
    Primary   string
    Secondary string
}

func ParseSchedule(filepath string) ([]Rotation, error)
func FindCurrentRotation(rotations []Rotation, now time.Time) (*Rotation, error)
func ValidateRotation(r Rotation) error
```

### Testing Approach

Table-driven tests in `schedule_test.go`:

**Parsing tests:**
- Valid schedule parsing
- Malformed timestamps
- Wrong field count
- Out of order timestamps
- Invalid email formats
- Empty file
- Duplicate emails in same rotation

**FindCurrentRotation tests (with mocked time):**
- Current time during a rotation period → returns that rotation
- Current time exactly at rotation boundary → returns that rotation
- Current time before first rotation → error
- Current time after last rotation → returns last rotation
- Timezone handling edge cases

## Google Groups API Integration

### Authentication

- Use Google Admin SDK Directory API
- Service account with domain-wide delegation
- Required scopes:
  - `https://www.googleapis.com/auth/admin.directory.group`
  - `https://www.googleapis.com/auth/admin.directory.group.member`
- Credentials provided via `GOOGLE_CREDENTIALS` environment variable (JSON content)

### API Operations

1. **List members** - `GET /admin/directory/v1/groups/{groupKey}/members`
   - Get current membership of the group
2. **Add member** - `POST /admin/directory/v1/groups/{groupKey}/members`
   - Add email with role="MEMBER"
3. **Remove member** - `DELETE /admin/directory/v1/groups/{groupKey}/members/{memberKey}`
   - Remove by email address

### Implementation

- Use official `google.golang.org/api/admin/directory/v1` package
- Wrap in a `GroupsClient` interface for testability
- Mock interface for unit tests (no real API calls)

```go
type GroupsClient interface {
    ListMembers(groupEmail string) ([]string, error)
    AddMember(groupEmail, memberEmail string) error
    RemoveMember(groupEmail, memberEmail string) error
}
```

### Error Handling

- Network failures → fail fast with clear error
- Authentication failures → fail with credential troubleshooting hints
- Group not found → fail with group name check
- Member already exists when adding → skip silently (idempotent)
- Member doesn't exist when removing → skip silently (idempotent)
- Rate limiting → fail fast (no retry for initial implementation)

## Sync Logic

### Reconciliation Process

1. **Fetch current state:**
   - Parse schedule file to get all rotations
   - Find current rotation based on current time
   - Query Google Group to get current members

2. **Calculate desired state:**
   - Desired members = [primary_email, secondary_email] from current rotation
   - If primary and secondary are the same person, only include once

3. **Calculate diff:**
   - To add = desired members not currently in group
   - To remove = current members not in desired set

4. **Apply changes:**
   - Execute removals first (clean up old oncall)
   - Execute additions second (add new oncall)
   - Each operation logs what it's doing

5. **Report results:**
   - Print summary of changes or "No changes needed"
   - Exit 0 on success, non-zero on any failure

### Code Structure

```go
type SyncResult struct {
    Added   []string
    Removed []string
}

func Sync(groupEmail string, configPath string, client GroupsClient, now time.Time) (*SyncResult, error)
```

### Testing Approach

Mock `GroupsClient` interface and test scenarios:
- Empty group → add both oncall people
- Correct members already → no changes
- Partial overlap → remove old, add new
- Complete mismatch → remove all, add both
- Same person as primary and secondary → add only once
- Verify correct API calls in correct order (remove then add)

## CLI Interface

### Command Usage

```bash
# GitHub Actions
GOOGLE_CREDENTIALS="${{ secrets.GOOGLE_SERVICE_ACCOUNT }}" \
  synconcall --config=dev.oncall --group=dev-oncall@bytebase.com

# Local testing
GOOGLE_CREDENTIALS="$(cat service-account.json)" \
  synconcall --config=dev.oncall --group=dev-oncall@bytebase.com
```

### Flags

- `--config` (required) - Path to dev.oncall schedule file
- `--group` (required) - Google Group email address to sync
- `--help` - Show usage information

### Environment Variables

- `GOOGLE_CREDENTIALS` (required) - Service account JSON key content

### Output Examples

**Success with changes:**
```
Reading schedule from: dev.oncall
Current rotation: 2026-01-12 - Primary: d@bytebase.com, Secondary: vh@bytebase.com
Syncing group: dev-oncall@bytebase.com
  Removed: old-person@bytebase.com
  Added: d@bytebase.com
  Added: vh@bytebase.com
Sync completed successfully.
```

**Success with no changes:**
```
Reading schedule from: dev.oncall
Current rotation: 2026-01-12 - Primary: d@bytebase.com, Secondary: vh@bytebase.com
Syncing group: dev-oncall@bytebase.com
  No changes needed.
Sync completed successfully.
```

**Error example:**
```
Error: failed to parse schedule: invalid timestamp format on line 3
```

### Exit Codes

- 0 = success
- 1 = any error

## Error Handling & Edge Cases

### Schedule File Errors

- File not found → `Error: config file not found: {path}`
- Parse errors → `Error: invalid schedule format on line {n}: {reason}`
- No rotations found → `Error: schedule file is empty`
- Current time before first rotation → `Error: no active rotation (current time is before first rotation start)`

### Google API Errors

- Missing GOOGLE_CREDENTIALS → `Error: GOOGLE_CREDENTIALS environment variable not set`
- Invalid JSON in credentials → `Error: invalid service account credentials format`
- Auth failure → `Error: authentication failed - check service account has domain-wide delegation`
- Group not found → `Error: group not found: {email}`
- Permission denied → `Error: service account lacks permission to manage group members`
- Network/API failures → `Error: Google API request failed: {details}`

### Edge Cases

- Current rotation has same person as primary and secondary → Add them only once to the group
- Person already in group → Skip adding (idempotent)
- Person not in group when removing → Skip removing (idempotent)
- Empty group to start → Just add the two current oncall people
- Duplicate emails in schedule file → Validation error during parse

### Logging

- All output to stdout for normal operation
- Errors to stderr
- Clear, actionable error messages

## Future Considerations

Items deferred for later:

1. Differentiate primary vs secondary oncall within Google Group
   - Current implementation treats both as regular "Member" role
   - Future: Could use custom member notes, roles, or separate groups

2. Retry logic for transient API failures
   - Current implementation fails fast
   - Future: Add exponential backoff for network/rate limit errors

3. Dry-run mode
   - Current implementation applies changes immediately
   - Future: Add `--dry-run` flag to preview changes
