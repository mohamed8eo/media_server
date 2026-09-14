# MediaServer

Self-hosted media library management system. Browse, stream, and organize your media collection with a modern web interface.

## Features

- **Upload & download** files and folders with drag-and-drop friendly HX triggers
- **YouTube & URL downloads** via yt-dlp
- **Media transcoding** — automatic thumbnail generation with ffmpeg/pdftoppm
- **Folder organization** with hierarchical nesting and rename/move operations
- **Trash & recycle bin** — soft delete with purge retention
- **Storage quotas** and usage statistics per user
- **JWT authentication** — access tokens (15 min) + refresh tokens (7 days)
- **HTMX-powered UI** — partial updates without full page reloads
- **Responsive design** — TailwindCSS + AlpineJS

## Quick Start

### Local development

```bash
# 1. Install dependencies
go install github.com/a-h/templ/cmd/templ@latest
go install github.com/air-verse/air@latest

# 2. Build and run
make build   # generates templ, builds binary
make run     # starts the API server

# 3. Watch for live reload
make watch   # starts Air
```

### First-time setup

```bash
# Using the CLI
mediaserver setup --project-dir /path/to/project \
  --data-path /path/to/data --port 8080

# Or via setup.sh
./setup.sh /path/to/install/dir
```

### Docker deployment (SSD)

```bash
# From project root
docker compose up --build
```

MediaServer runs at `http://localhost:8080` (configurable via `PORT` env var).

## CLI Reference

| Command | Description |
|---|---|
| `mediaserver setup` | Install binary + config + apply migrations |
| `mediaserver start` | Start Docker compose |
| `mediaserver stop` | Stop Docker compose |
| `mediaserver status` | Show status + autostart state |
| `mediaserver update` | Git pull + rebuild + restart |
| `mediaserver remove` | Remove Docker containers |
| `mediaserver remove --purge-data --force` | Delete all persistent data |
| `mediaserver config get` | Show current configuration |
| `mediaserver config set port 9090` | Change port |
| `mediaserver config set data-path /new/path` | Change data directory |
| `mediaserver autostart enable/disable` | Systemd user service |

See [`docs/cli-reference.md`](./docs/cli-reference.md) for full details.

## API Reference

Full endpoint documentation available at [`docs/api-reference.md`](./docs/api-reference.md).

Key endpoints:

- `POST /` — Upload file
- `GET /` — List files and folders
- `GET /{id}` — Get file metadata
- `DELETE /{id}` — Soft delete to trash
- `POST /trash/restore` — Restore from trash
- `POST /trash/purge` — Permanent deletion
- `POST /download-url` — Add URL download (yt-dlp)
- `GET /recent` — Recent uploads and playback
- `GET /stats` — Storage statistics

## Deployment

- **Docker Compose** — `make docker-run` / `make docker-down`
- **SSD portable** — Install via `mediaserver setup` or `./setup.sh`
- **Systemd autostart** — `mediaserver autostart enable`
- See [`docs/deployment.md`](./docs/deployment.md) for detailed guides.

## Development

- **Testing** — `make test` runs all tests
- **Live reload** — `make watch` starts Air
- **SQLC migrations** — `make sqlc` regenerates query code
- See [`docs/development.md`](./docs/development.md) for architecture details.

---

*MediaServer is actively developed. Check the [issues](https://github.com/your-repo/issues) for roadmap.*