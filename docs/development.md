# Development Guide

This guide is for developers extending or maintaining MediaServer.

## Project Architecture

### High-level overview

```
cmd/api/main.go        # HTTP server entry point
cmd/mediaserver/       # CLI command manager
cmd/web/               # templ frontend templates
internal/database/     # SQLC queries, migrations, DB service
internal/files/        # File upload/download/thumbnail handlers
internal/auth/         # JWT authentication, user routes
internal/jobqueue/     # Background job worker pool
internal/middleware/   # HTTP middleware (auth, CORS, security)
internal/models/       # Data models (User, File, Folder, RefreshToken)
internal/server/       # HTTP server configuration + route registration
internal/utils/        # Response helpers, format functions
```

### Data flow

1. **HTTP request** → `internal/server/routes.go` registers routes
2. **Middleware** — auth checks JWT, CORS headers, security headers
3. **Handler** — `internal/files/routes.go` or `internal/auth/routes.go`
4. **Database** — sqlc-generated queries from `internal/database/sqlc/`
5. **Job queue** — background ffmpeg/yt-dlp processing via `internal/jobqueue/pool`
6. **Response** — JSON or HX-triggered partial updates

---

## Build System

### Makefile targets

| Target | Description |
|---|---|
| `make all` | Build + test (templ-install + build + test) |
| `make build` | Install templ + generate + build binary |
| `make run` | `go run cmd/api/main.go` |
| `make test` | `go test ./... -v` |
| `make watch` | Start Air live reload |
| `make docker-run` | Docker compose up --build |
| `make docker-down` | Docker compose down |
| `make clean` | Remove built binary |
| `make sqlc` | Run sqlc code generation |

### Manual build workflow

```bash
# 1. Install templ if not present
go install github.com/a-h/templ/cmd/templ@latest

# 2. Generate templ components
templ generate

# 3. Generate sqlc queries
make sqlc   # or: sqlc generate

# 4. Build the binary
CGO_ENABLED=1 GOOS=linux go build -o main cmd/api/main.go

# 5. Run
./main   # or: go run cmd/api/main.go
```

### sqlc code generation

The project uses sqlc for type-safe SQL queries.

- **Configuration:** `sqlc.yaml`
- **Schema:** `internal/database/migrations/` (embedded)
- **Queries:** `db/query/*.sql` (raw SQL)
- **Generated:** `internal/database/sqlc/` (Go types & queries)

Run `make sqlc` to regenerate after schema changes.

---

## Database Schema

MediaServer uses SQLite with goose migrations. Three migration files:

| Migration | Purpose |
|---|---|
| `00001_init_schema.sql` | Users, refresh_tokens, files tables |
| `00002_folders_and_trash.sql` | Folders table, soft delete (deleted_at) |
| `00003_jobs.sql` | Jobs table for background tasks |

### Connection configuration

`BLUEPRINT_DB_URL` format:
```
file:./db/mediavault.db?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000
```

Configure via `.env` file or `mediaserver config set`.

### SQLC query locations

- `db/query/users.sql`
- `db/query/files.sql`
- `db/query/jobs.sql`
- `db/query/refresh_tokens.sql`
- `db/query/reset.sql`

---

## File Storage

### Directory structure

```
<STORAGE_PATH>/
├── db/
│   └── mediavault.db
└── storage/
    └── <user-id>/
        └── <file-uuid>.<ext>
```

### User isolation

Each user gets a dedicated directory under `STORAGE_PATH`:
```
<STORAGE_PATH>/<user-uuid>/<file-uuid>.<ext>
```

Path traversal is prevented in `internal/files/routes.go:156` by verifying
`absTarget` starts with `absUser`.

### Media processing pipeline

1. **Upload** — file saved to user directory
2. **Thumbnail** — if video: ffmpeg generates JPEG; if image: direct copy
3. **Job queue** — `jobqueue.Pool` processes transcoding asynchronously
4. **Progress** — `UpdatePlaybackProgressHandler` updates DB playback progress

#### Thumbnail generation (`internal/files/thumbnail.go`)

- Video files: uses `ffmpeg` to extract frame at 10%
- Image files: copies using `mimetype` library
- PDF files: uses `pdftoppm`

---

## Job Queue System

### Pool implementation (`internal/jobqueue/pool.go`

```go
type Pool struct {
    jobs chan func()
}

func NewPool(worker, queueSize int) *Pool
func (p *Pool) Submit(job func())
func (p *Pool) SubmitWait(job func())  // blocking wait for completion
```

### Configuration

- **Workers:** `runtime.NumCPU()` (for audio pool)
- **Queue size:** 50 jobs per pool
- **Two pools:** `audioPool` (transcoding) and `thumbPool` (thumbnails)

### Job types

| Task type | Description |
|---|---|
| `media_fix` | FFmpeg transcoding + thumbnail generation |
| (custom) | Extendable via job queue |

### Job lifecycle

1. Created via `database.CreateJob(jobID, fileID, userID, "media_fix")`
2. Submitted to pool → status updated to `processing`
3. On completion: status → `completed` or `failed` with error message
4. Visible via `GET /jobs/{id}` endpoint

---

## Authentication

### JWT configuration

- **Access token:** 15 minutes duration
- **Refresh token:** 7 days duration
- **Signing method:** HS256

### Token generation (`internal/auth/jwt.go`)

```go
const (
    AccessTokenDuration  = 15 * time.Minute
    RefreshTokenDuration = 7 * 24 * time.Hour
)
```

### Middleware

`internal/middleware/auth.go` — validates JWT from `Authorization: Bearer <token>`
header and sets `middleware.UserIDKey` in request context.

### Auth routes

- `POST /auth/register` — Create new user
- `POST /auth/login` — Authenticate and receive tokens
- `POST /auth/refresh` — Refresh access token using refresh token

---

## Testing

### Test organization

```
internal/database/      # DB unit tests
internal/middleware/    # Auth + security tests
internal/files/         # Route + comprehensive tests
cmd/mediaserver/        # CLI unit tests
```

### Running tests

```bash
make test   # go test ./... -v
```

Or run specific packages:

```bash
go test ./internal/... -v
go test ./cmd/... -v
```

### Test categories

- **Unit tests:** Individual function behavior
- **Integration tests:** Database interactions (uses test SQLite)
- **Route tests:** HTTP endpoint verification
- **E2E tests:** `internal/server/e2e_refresh_manual_test.go`

### Writing tests

Follow existing patterns:

```go
func TestMyFunction(t *testing.T) {
    // Arrange
    // Act
    // Assert
}
```

Use `t.Setenv()` for environment isolation, `t.TempDir()` for temp paths.

---

## Templ Development

### Adding a new component

1. Create `internal/web/*.templ` with HTML + templ directives
2. Create `internal/web/*_templ.go` with Go template data
3. Run `templ generate` to compile
4. Use the generated component in other templates

### Templ syntax basics

- Components: `<Card title="Hello">…</Card>`
- Parameters: `<Card title="Hello">content</Card>` or `<Card title="Hello" />
- Loops: range over Go slices/maps
- Conditionals: if/else expressions

---

## Frontend Development

### HTMX patterns used

| Pattern | Selector | Description |
|---|---|---|
| HX trigger | `[hx-trigger]` | Declarative event listeners |
| HX swap | `[hx-swap]` | How to replace DOM |
| HX target | `[hx-target]` | Where to inject response |
| HX push | `[hx-push]` | History API integration |

### AlpineJS global store

Accessible as `window.Alpine.data('app')` or via `x-data` on `.app-shell`.
Provides reactive state: `sidebarOpen`, `searchQuery`, `selectedItems`, etc.

### TailwindCSS utilities

Most UI styling uses Tailwind utility classes directly in templ files.
No custom CSS required for standard components.

---

## Common Development Tasks

### Adding a new file category

1. Update `internal/models/file.go` `Category()` method if needed
2. Update `internal/models/file.go` `TypeBadge()` method
3. Add category handling in `internal/server/routes.go` if new endpoints

### Changing database schema

1. Add migration in `internal/database/migrations/`
2. Update SQL in `db/query/*.sql`
3. Run `make sqlc` to regenerate Go queries
4. Update `Service` interface in `internal/database/database.go` if new methods needed

### Adding a new API endpoint

1. Add route in `internal/server/routes.go`
2. Add handler method in `internal/files/routes.go` or `internal/auth/routes.go`
3. Add templ component if UI needed
4. Add endpoint documentation in `docs/api-reference.md`
5. Add test in relevant test file

### Debugging media processing

1. Check job status via `GET /jobs/{id}`
2. View server logs (`make watch` for live logs)
3. Verify ffmpeg is installed and accessible
4. Check `internal/files/thumbnail.go` for error output

---

## Environment Variables Reference

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP server port |
| `APP_ENV` | `local` | Environment (local/prod) |
| `BLUEPRINT_DB_URL` | `file:./db/mediavault.db?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000` | SQLite connection |
| `JWT_SECRET` | Generated at first run | Secret for signing JWT tokens |
| `STORAGE_PATH` | `./storage` | Base directory for user uploads |
| `MEDIASERVER_CONFIG` | — | Override config.json path |

---

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/foo`)
3. Make changes following existing code patterns
4. Run `make test` to verify no regressions
5. Run `make sqlc` if database changes
6. Commit with descriptive message
7. Open a Pull Request

### Code style

- Go: `gofmt` standard, `go vet` clean
- templ: Follow component conventions in `cmd/web/`
- SQL: Match existing style in `db/query/*.sql`
- JavaScript/HTML: Match existing templ component patterns