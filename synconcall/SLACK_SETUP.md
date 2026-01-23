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

     > [!IMPORTANT]  
     > **DO NOT** select scopes starting with `admin.` (e.g., `admin.usergroups:write`). These require an Enterprise Grid plan and will cause installation errors. Ensure you select the standard `usergroups:write`.

3. **Install App & Get Token**
   - Scroll up to **OAuth Tokens for Your Workspace**.
   - Click **Install to Workspace**.
   - Allow the permissions.
   - Copy the **User OAuth Token**. It should start with `xoxp-`.

## Usage

Run the tool with your new token:

```bash
export SLACK_TOKEN="xoxp-your-token-here"
go run ./synconcall --config=dev.oncall --slack-group=S0AAPDZBNQL
```
