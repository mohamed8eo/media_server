# Deployment

This guide covers deploying MediaServer in various environments.

## Deployment Methods

### 1. Docker Compose (recommended for server deployment)

From the project root:

```bash
make docker-run
```

This builds the image and starts the container with SQLite database and
storage volume mounted.

**Port:** Configure via `PORT` env var or `mediaserver config set port`.

**Volumes:**
- `${DATA_DIR:-./data}/db` — SQLite database persistence
- `/storage` — User file uploads (bind mount or named volume)

**Environment variables in docker-compose.yml:**

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP server port |
| `APP_ENV` | `local` | Environment name |
| `BLUEPRINT_DB_URL` | SQLite file:./db/mediavault.db... | Database connection string |
| `STORAGE_PATH` | `./storage` | Base path for user uploads |
| `TLS_CERT_FILE` | `/app/localhost-cert.pem` | Path to TLS certificate PEM file for HTTPS |
| `TLS_KEY_FILE` | `/app/localhost-key.pem` | Path to TLS private key PEM file for HTTPS |

**HTTPS Support in Docker:**
To run the Docker container directly on HTTPS, mount your local certificate and key files into the container via `docker-compose.yml`:
```yaml
environment:
  TLS_CERT_FILE: /app/localhost-cert.pem
  TLS_KEY_FILE: /app/localhost-key.pem
volumes:
  - ./localhost-cert.pem:/app/localhost-cert.pem:ro
  - ./localhost-key.pem:/app/localhost-key.pem:ro
```

To use an external SSD:

```bash
docker compose up --build \
  -e DATA_DIR=/mnt/mediaserver-ssd \
  -e STORAGE_PATH=/mnt/mediaserver-ssd/media-data
```

---

### 2. SSD Portable Deployment

Install MediaServer to an external SSD for maximum portability.

```bash
./setup.sh /path/to/ssd/mediavault
```

Or via CLI:

```bash
go install ./cmd/mediaserver
mediaserver setup \
  --project-dir /mnt/mediaserver-ssd/mediaserver \
  --data-path /mnt/mediaserver-ssd/media-data \
  --port 8080
```

**What this sets up:**

- Binary at `<project-dir>/mediavault`
- `.env` file with configuration
- SQLite database at `<data-dir>/db/mediavault.db`
- User storage at `<data-dir>/storage/`
- Optional systemd user service for autostart

**To start:**

```bash
mediaserver start
```

**To access:** `http://<host-ip>:8080`

**Backup:** Copy `<data-dir>/` directory — contains database and all uploaded files.

---

### 3. Systemd Autostart

Enable automatic startup on login:

```bash
mediaserver autostart enable
```

This installs a systemd user service that:
- Waits for both project and data mounts
- Starts MediaServer Docker compose
- Restarts on failure after 30 seconds

Disable with:

```bash
mediaserver autostart disable
```

The service unit file is stored at
`~/.config/systemd/user/mediaserver.service`.

---

### 4. Docker Swarm / Kubernetes

For production orchestration, the Docker Compose file can be adapted.
Key configuration:

```yaml
services:
  app:
    image: mediaserver:latest
    ports:
      - "${PORT:-8080}:8080"
    environment:
      - PORT=8080
      - BLUEPRINT_DB_URL=file:/db/mediavault.db?...
      - STORAGE_PATH=/storage
    volumes:
      - db-data:/db
      - storage-data:/storage
    restart: unless-stopped

volumes:
  db-data:
  storage-data:
```

---

## Configuration Persistence

### `.env` file

Non-secret deployment settings are stored in `~/.config/mediaserver/config.json`.
Application secrets (like `JWT_SECRET`) remain in the local `.env` file.

### Config.json location

```
~/.config/mediaserver/config.json
```

Overridden by `MEDIASERVER_CONFIG` environment variable if set.

---

## Updating MediaServer

### Via CLI

```bash
mediaserver update
```

This:
1. Checks git working tree is clean
2. Pulls latest changes
3. Rebuilds the binary
4. Restarts the application

### Via Docker

```bash
make docker-run
# Or:
docker compose up --build
```

### Manual rebuild

```bash
make clean
make build
# Then restart the server
```

---

## Rollback

If an update introduces issues:

```bash
mediaserver update  # or git checkout <previous-commit> && mediaserver update
```

Or using Docker:

```bash
docker compose down
docker compose up  # Use existing image tags
```

---

## Security Considerations

- Change `JWT_SECRET` in `.env` after initial setup
- Restrict `STORAGE_PATH` to a dedicated mounted filesystem
- Use HTTPS reverse proxy (Traefik, Nginx) for production
- Set proper file permissions on `<data-dir>/storage/`
- Regularly backup `<data-dir>/db/` and `<data-dir>/storage/`