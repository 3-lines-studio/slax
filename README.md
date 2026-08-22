# slax

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
SLAX_WORKDIR=/path/to/project \
slax
```

Optional:

```sh
SLAX_AX_PATH=/path/to/ax
SLAX_BASE_URL=https://api.deepseek.com
SLAX_MODEL=deepseek-v4-flash
SLAX_SYSTEM_FILE=/path/to/prompt.md
SLAX_SESSION_DIR=/path/to/sessions
```

Slax passes `AX_SLACK_CHANNEL` and `AX_SLACK_THREAD` to AX tool processes.

Sessions are stored under `~/.config/slax/sessions` or the platform config directory. Slax processes one AX request at a time.

## Build and test

```sh
go test ./...
go build ./...
```
