# Slaxi

Slack interface for AX through Axis. Each Slack thread maps to one persistent Axis session.

## Requirements

- Axis and Slaxi in `PATH`
- a Slack app with Socket Mode enabled
- `app_mentions:read`, `im:history`, `chat:write`, and `files:write` bot scopes
- `app_mention` and `message.im` event subscriptions

## Axis mode

```sh
export SLACK_APP_TOKEN=xapp-...
export SLACK_BOT_TOKEN=xoxb-...
export SLAXI_AXIS_URL=http://127.0.0.1:8081
export SLAXI_AXIS_USERNAME=axi
export SLAXI_AXIS_PASSWORD=secret
export SLAXI_BOT_ID=alfred
export SLAXI_PROJECT_ID=alfred
slaxi
```

Slaxi creates Axis sessions, streams responses, and uploads Axis artifact events to the originating Slack thread. When Axis supervises Slaxi, connector definitions provide the bot and project while `~/.config/axis/connectors/<connector-id>.env` provides the Slack tokens.

Thread mappings are stored under `~/.config/slaxi/sessions` by default. Set `SLAXI_SESSION_DIR` to change it.

## Legacy direct mode

Without `SLAXI_AXIS_URL`, Slaxi retains its direct AX subprocess mode:

```sh
SLACK_APP_TOKEN=xapp-... \
SLACK_BOT_TOKEN=xoxb-... \
SLAXI_WORKDIR=/path/to/project \
slaxi
```

Optional direct-mode variables:

```text
SLAXI_AX_PATH
SLAXI_BASE_URL
SLAXI_MODEL
SLAXI_SYSTEM_FILE
```

## Test

```sh
go test ./...
```
