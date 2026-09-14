# Troubleshooting

Common issues and solutions for MediaServer.

## Port Already in Use

### Symptoms
- Server fails to start
- `listen: addr already in use` error in logs

### Causes
- Another process using the configured port (default 8080)
- Previous server instance still running

### Solutions

```bash
# Check what's using the port
lsof -i :8080
# Or
ss -tlnp | grep 8080

# Kill the process
kill <PID>

# Or change the port
mediaserver config set port 9090
```

---

## Database Locked / SQLite Errors

### Symptoms
- `database is locked` errors
- Migrations fail on first run
- `goose: unable to start a migration transaction`

### Causes
- Multiple processes writing to the same SQLite database without WAL mode
- Long-running transactions not committed

### Solutions

```bash
# Ensure WAL mode is enabled (default in this project)
# The BLUEPRINT_DB_URL includes _journal_mode=WAL

# If database is corrupted, remove and restart
rm ./db/mediavault.db
# Then restart the server — migrations will re-apply

# Increase busy timeout if getting lock errors
# Set in .env: BLUEPRINT_DB_URL=file:./db/mediavault.db?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=10000
```

---

## Permission Denied on Storage Path

### Symptoms
- `Failed to prepare storage` or `Failed to save file`
- Cannot upload files
- Permission errors on `<STORAGE_PATH>/`

### Causes
- `STORAGE_PATH` not writable
- Missing directory permissions
- Path traversal protection blocking access

### Solutions

```bash
# Verify path is writable
ls -la /path/to/storage
chmod 755 /path/to/storage

# Ensure STORAGE_PATH is an absolute path in .env
# And that the directory exists and is owned by the running user

# Check for path traversal blocks in logs
# Valid paths must be under the user's storage directory
```

---

## Thumbnail Generation Failures

### Symptoms
- `thumbnail generation failed` in logs
- `Thumbnail not available` shown in UI
- `File not found` when trying to view thumbnails

### Causes
- ffmpeg not installed or not in PATH
- Unsupported media format
- File permissions on thumbnail path

### Solutions

```bash
# Verify ffmpeg is installed
ffmpeg -version

# Check if the file type is supported
# Videos: ffmpeg-supported formats
# Images: JPEG, PNG, GIF, WebP
# PDFs: pdftoppm required

# Check logs for specific error
# Thumbnail path: <storage>/<uuid>/thumb.jpg
```

---

## Docker Compose Issues

### Symptoms
- `docker compose up` fails
- Containers exit immediately
- Volume mount errors

### Causes
- Docker not running
- Port conflicts on host
- Missing external SSD mount paths

### Solutions

```bash
# Start Docker daemon
# Ensure Docker is running: dockerd or Docker Desktop

# Check container logs
docker compose logs app

# If using SSD deployment, ensure paths exist
mkdir -p /mnt/mediaserver-ssd/db /mnt/mediaserver-ssd/storage

# Restart with fresh state
make docker-down
make docker-run
```

---

## Authentication Failures

### Symptoms
- `401 Unauthorized` on all endpoints
- Cannot log in
- Tokens not accepted

### Causes
- Missing or invalid `JWT_SECRET` in `.env`
- Clock skew between token issuance and validation
- Token expired (access tokens: 15 min, refresh tokens: 7 days)

### Solutions

```bash
# Regenerate JWT_SECRET
# Delete the .env file and run setup again
rm ./.env
mediaserver remove
mediaserver setup

# Check token expiration
# Access tokens last 15 minutes — use refresh token for new access token
# Refresh tokens last 7 days
```

---

## Git Update Failures

### Symptoms
- `mediaserver update` refuses with "Git working tree is not clean"
- Pull fails with merge conflicts

### Causes
- Uncommitted changes in the project
- Local modifications not staged

### Solutions

```bash
# Stash local changes
git stash

# Or commit them
git add .
git commit -m "save local changes"

# Then update
mediaserver update

# To restore stashed changes after update
git stash pop
```

---

## Media Processing Stuck

### Symptoms
- Job status stays at `processing` indefinitely
- Thumbnail never appears
- File upload succeeds but transcoding doesn't complete

### Causes
- Job queue pool exhausted (max 50 pending per pool)
- ffmpeg errors on specific media files
- Database connection issues

### Solutions

```bash
# Check job status
curl http://localhost:8080/jobs/<job-uuid>

# View server logs
# With Air: check terminal running `make watch`
# Without: check stderr output

# Reset stuck jobs manually (SQL)
# Or restart the server to clear the pool

# Verify ffmpeg can process the file
ffmpeg -i /path/to/file -vframes 1 -f image2 /tmp/thumb.jpg
```

---

## Empty Trash Fails

### Symptoms
- `Failed to empty recycle bin` error
- Items remain in trash after emptying

### Causes
- File already deleted from filesystem
- Permission issues removing files
- Database inconsistencies

### Solutions

```bash
# Check if files exist physically
ls -la <STORAGE_PATH>/<user-id>/<file-uuid>.*

# Manual cleanup
# Remove orphaned files from storage
# Re-run empty trash

# Check logs for specific OS error
```

---

## Performance Issues

### Symptoms
- Slow page loads
- High CPU usage
- Slow file operations

### Solutions

```bash
# Check database connection stats
# With server running, visit /stats endpoint

# Optimize SQLite
# Ensure WAL mode is used (default)
# Set appropriate busy_timeout

# Limit concurrent uploads
# maxUploadSize in internal/files/routes.go:27 is 10GB max

# Monitor job queue
# GET /jobs/{id} to check processing status

# Consider increasing worker pool sizes in internal/jobqueue/pool.go
```

---

## Getting Help

If your issue isn't covered here:

1. Check the [GitHub Issues](https://github.com/your-repo/issues) for known problems
2. Run `make test` to verify no regressions
3. Check server logs with `make watch` or `go run cmd/api/main.go`
4. Open a new issue with:
   - OS and version
   - MediaServer version (`go version` + git describe)
   - Steps to reproduce
   - Relevant log output
   - Environment variables (`cat .env`)