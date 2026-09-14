# CLI Reference

The `mediaserver` CLI command manages self-hosted deployments. All commands are
subcommands of `mediaserver <command>`.

## Installation

```bash
go install ./cmd/mediaserver
# Or from compiled binary in the project
```

## Global Configuration

The CLI stores configuration in `~/.config/mediaserver/config.json`. Non-secret
deployment settings (project dir, data path, port) are persisted here. Secrets
remain in the local `.env` file.

### Config.json format

```json
{
  "projectDir": "/mnt/mediaserver-ssd/mediaserver",
  "dataDir": "/mnt/mediaserver-ssd/media-data",
  "port": 8080
}
```

Environment variables override config.json values when both are set.

---

## Commands

### `mediaserver setup`

Initialize a new MediaServer installation. Prompts for or accepts paths and port.

**Flags:**

| Flag | Default | Description |
|---|---|---|
| `--project-dir` | `/mnt/mediaserver-ssd/mediaserver` | Project root directory |
| `--data-path` | `<project-dir>/data` | Persistent data directory |
| `--port` | `8080` | HTTP server port |

**What it does:**

1. Validates that paths are absolute and not `/`
2. Creates `<data-dir>/db/` and `<data-dir>/storage/` directories
3. Generates a `.env` file with a random `JWT_SECRET` if one doesn't exist
4. Runs the binary once to apply database migrations
5. Installs the binary via `go install`
6. Optionally installs a systemd user service for autostart

**Example:**

```bash
mediaserver setup \
  --project-dir /mnt/mediaserver-ssd/mediaserver \
  --data-path /mnt/mediaserver-ssd/media-data \
  --port 8080
```

---

### `mediaserver start`

Start the Docker compose stack.

```bash
mediaserver start
```

Equivalent to `docker compose up -d --build` in the project directory.

---

### `mediaserver stop`

Stop the Docker compose stack.

```bash
mediaserver stop
```

Equivalent to `docker compose stop` in the project directory.

---

### `mediaserver status`

Show current deployment status.

```bash
mediaserver status
```

Outputs:

- Project directory
- Data directory
- Port
- Autostart enabled/disabled state (via `systemctl --user is-enabled`)
- Docker container status (`docker compose ps`)

---

### `mediaserver update`

Pull latest changes and rebuild.

```bash
mediaserver update
```

Steps:

1. Verifies git working tree is clean (`git status --porcelain`)
2. Runs `git pull --ff-only`
3. Restarts the application (`mediaserver start`)

Fails if the working tree has uncommitted changes.

---

### `mediaserver remove`

Remove Docker containers and optionally data.

**Flags:**

| Flag | Description |
|---|---|
| `--purge-data` | Delete the entire data directory |
| `--force` | Confirm data deletion (required with `--purge-data`) |

**Behavior:**

- Always runs `docker compose down --remove-orphans`
- If `--purge-data` is set AND `--force` is set: removes `<data-dir>` entirely
- If `--purge-data` is set but `--force` is not: error `--purge-data requires --force`
- Keeps the database and upload files by default

**Example — safe removal (containers only):**

```bash
mediaserver remove
```

**Example — complete removal:**

```bash
mediaserver remove --purge-data --force
```

---

### `mediaserver config get`

Print the current configuration (from config.json) as formatted JSON.

```bash
mediaserver config get
```

---

### `mediaserver config set`

Set a configuration value and optionally restart.

**Flags:**

| Key | Description |
|---|---|
| `port <port>` | Change HTTP port (validates availability) |
| `data-path <path>` | Change data directory (requires empty destination) |

**Example:**

```bash
mediaserver config set port 9090
mediaserver config set data-path /new/path/to/data
```

---

### `mediaserver autostart`

Enable or disable the systemd user service for automatic startup.

**Flags:**

| Value | Description |
|---|---|
| `enable` | Install and enable the systemd service |
| `disable` | Disable and remove the systemd service |

**Example:**

```bash
mediaserver autostart enable
mediaserver autostart disable
```

---

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP server port |
| `APP_ENV` | `local` | Environment name |
| `BLUEPRINT_DB_URL` | `file:./db/mediavault.db?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000` | SQLite connection URL |
| `JWT_SECRET` | Generated at first run | Secret for JWT token signing |
| `STORAGE_PATH` | `./storage` | Base directory for user file uploads |

The CLI reads `MEDIASERVER_CONFIG` env var to override the config.json path.