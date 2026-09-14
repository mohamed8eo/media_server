# Getting Started

This guide will help you set up MediaServer for local development and first-time use.

## Prerequisites

The following tools are required:

| Tool | Minimum Version | Purpose |
|---|---|---|
| Go | 1.21+ | Runtime and build |
| templ | latest | HTML component compiler |
| ffmpeg | 4.0+ | Media thumbnail generation |
| air (optional) | latest | Live reload during development |
| sqlite3 (optional) | 3.35+ | Database inspection |

## Installation

### Option 1: Using Make (recommended)

```bash
# Clone the repository
git clone https://github.com/your-repo/mediaserver.git
cd mediaserver

# Install templ code generator
make templ-install

# Build the application (also runs templ generate)
make build

# Start the server
make run
```

### Option 2: Manual installation

```bash
# Install templ
go install github.com/a-h/templ/cmd/templ@latest

# Install air (for live reload)
go install github.com/air-verse/air@latest

# Build the binary
CGO_ENABLED=1 GOOS=linux go build -o main cmd/api/main.go
```

## Environment Configuration

Copy `.env` from the example or let the system generate it:

```bash
# The setup.sh script creates .env automatically
# Or manually create .env in project root:
cat > .env << 'EOF'
PORT=8080
APP_ENV=local
BLUEPRINT_DB_URL=file:./db/mediavault.db?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000
STORAGE_PATH=./storage
JWT_SECRET=change-this-to-a-secure-random-value
EOF
```

## Database Setup

MediaServer uses SQLite with goose migrations. On first run, the `New()` function in `internal/database/database.go` automatically applies all migrations found in `internal/database/migrations/`.

The default database URL expects a file-based SQLite database:
```
BLUEPRINT_DB_URL=file:./db/mediavault.db?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000
```

## First Run

1. Ensure `STORAGE_PATH` directory exists and is writable
2. Run the server: `go run cmd/api/main.go`
3. The server will start and apply any pending migrations
4. Open `http://localhost:8080` in your browser

## Development Workflow

### With Air (live reload)

```bash
make watch
```

Air will watch for file changes and automatically rebuild/restart the server.
Templ components are also watched and recompiled.

### Manual development

```bash
# Run the server
go run cmd/api/main.go

# In another terminal, watch for templ changes
templ generate
```

## Testing

Run the full test suite:

```bash
make test
```

Or run specific test packages:

```bash
go test ./internal/... -v
```

## Docker Development

```bash
# Build and start Docker compose
make docker-run

# Stop
make docker-down

# View logs
docker compose logs -f app
```

## Directory Structure After Setup

```
your-project/
├── cmd/          # Entry points
├── internal/     # Application logic
├── data/
│   ├── db/       # SQLite database
│   └── storage/  # User file uploads
├── .env          # Environment variables
├── .env.json     # CLI config persistence
└── docker-compose.yml
```

---

Need help? Check the [troubleshooting guide](./docs/troubleshooting.md) or [CLI reference](./docs/cli-reference.md).