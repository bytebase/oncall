# Slack Sync Setup Guide

To sync on-call rotations to a Slack User Group, you need a **Slack User OAuth Token**.

## Prerequisites
- You must be a **Workspace Admin** or **Owner** to manage User Groups.
- Your workspace must be on a paid Slack plan (Standard/Plus/Enterprise) to use User Groups.

## Steps to Create Token

1. **Create a Slack App**
   - Go to [Slack API: Your Apps](https://api.slack.com/apps).
   - Click **Create New App**.
   - Select **From scratch**.
   - Name it `On-Call Sync` and select your workspace.

2. **Configure Scopes**
   - In the left sidebar, click **OAuth & Permissions**.
   - Scroll down to **User Token Scopes** (NOT Bot Token Scopes).
   - Add the following scopes:
     - `usergroups:read`  (To list group members)
     - `usergroups:write` (To add/remove members)
     - `users:read`       (To look up user details)
     - `users:read.email` (To look up users by email)
     - `chat:write`       (To post notification messages)

     > [!IMPORTANT]  
     > **DO NOT** select scopes starting with `admin.` (e.g., `admin.usergroups:write`). These require an Enterprise Grid plan and will cause installation errors. Ensure you select the standard `usergroups:write`.

3. **Install App & Get Token**
   - Scroll up to **OAuth Tokens for Your Workspace**.
   - Click **Install to Workspace**.
   - Allow the permissions.
   - Copy the **User OAuth Token**. It should start with `xoxp-`.

## Finding IDs

### Finding User Group ID
1. Open Slack on desktop.
2. Go to **People & User Groups** in the sidebar.
3. Click on the desired User Group (e.g., `@dev-oncall`).
4. Click the **...** (three dots) menu near the top right of the group card.
5. Select **Copy ID**. It usually starts with `S` (e.g., `S0123456789`).

### Finding Channel ID
1. Open the desired channel in Slack.
2. Click on the **channel name** in the header to open details.
3. Scroll to the bottom of the "About" tab.
4. You will see **Channel ID**. It usually starts with `C` (e.g., `C0123456789`).

## Usage

Run the tool with your new token:

```bash
export SLACK_TOKEN="xoxp-your-token-here"
# Sync + Notify
go run ./synconcall \
    --config=dev.oncall \
    --slack-group=S0AAPDZBNQL \
    --slack-channel=C012345ABC
```
