# slaxi

A small Slack adapter for [AX](https://github.com/3-lines-studio/ax).
Each Slack thread maps to one persistent AX session.

## Requirements

- `ax` in `PATH` with `--session` support
- a Slack app with Socket Mode enabled
- `app_mentions:read`, `im:history`, and `chat:write` bot scopes
- `app_mention` and `message.im` event subscriptions

## Run

```sh
SLACK_APP_TOKEN=xapp-... \
SLACK_BOT_TOKEN=xoxb-... \
SLAXI_WORKDIR=/path/to/project \
slaxi
```

Optional:

```sh
SLAXI_AX_PATH=/path/to/ax
SLAXI_BASE_URL=https://api.deepseek.com
SLAXI_MODEL=deepseek-v4-flash
SLAXI_SYSTEM_FILE=/path/to/prompt.md
SLAXI_SESSION_DIR=/path/to/sessions
```

Slaxi passes `AX_SLACK_CHANNEL` and `AX_SLACK_THREAD` to AX tool processes.

Sessions are stored under `~/.config/slaxi/sessions` or the platform config directory. Slaxi processes one AX request at a time.

## Build and test

```sh
go test ./...
go build ./...
```
