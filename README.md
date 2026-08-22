# slax

A small Slack adapter for [AX](https://github.com/3-lines-studio/ax).
Each Slack thread maps to one persistent AX session.

## Requirements

- `ax` in `PATH` with `--session` support
- a Slack app with Socket Mode enabled
- `app_mentions:read` and `chat:write` bot scopes
- an `app_mention` event subscription

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
```

Sessions are stored under `~/.config/slax/sessions` or the platform config directory. Slax processes one AX request at a time.

## Build and test

```sh
go test ./...
go build ./...
```
