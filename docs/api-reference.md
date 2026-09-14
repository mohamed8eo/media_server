# API Reference

MediaServer uses a Chi-based HTTP API with JWT authentication. Most endpoints
require an authenticated user context set by the `auth` middleware.

## Base URL

```
http://localhost:8080
```

(Port configurable via `PORT` env var or `mediaserver config set port`.)

---

## Authentication

Authenticate by obtaining tokens via the auth API, then include the access token
in the `Authorization: Bearer <token>` header for protected routes.

### Token types

| Type | Duration | Purpose |
|---|---|---|
| Access token | 15 minutes | Standard API access |
| Refresh token | 7 days | Obtain new access token |

### Login / Register

```
POST /auth/register
POST /auth/login
```

Both return:

```json
{
  "id": "user-uuid",
  "email": "user@example.com",
  "created_at": "2024-01-01T00:00:00Z"
}
```

Together with access and refresh tokens in the response.

### Refresh token

```
POST /auth/refresh
```
Body: `{ "refresh_token": "<token>" }`
Returns new access token.

---

## File Endpoints

All file endpoints require authentication unless noted otherwise.

### Upload file

```
POST /
```

**Multipart form data:**

| Field | Description |
|---|---|
| `file` | The file to upload |
| `folder` (optional) | Destination folder path |

**Success response:**

```json
{
  "id": "file-uuid",
  "filename": "example.mp4",
  "size": "12345678"
}
```

**HX Trigger:** `mediaUpdated` — signals UI to refresh media list.

---

### List files and folders

```
GET /
```

**Query parameters:** None required.

**Success response:**

```json
{
  "files": [...],
  "folders": ["/", "/photos", "/videos"],
  "jobs": []
}
```

---

### Get file metadata

```
GET /{id}
GET /{id}/{filename}
```

**URL parameters:**

| Parameter | Description |
|---|---|
| `id` | File UUID |
| `filename` | Optional original filename |

**Success response:** File metadata object.

**Auth:** Required — checks `file.UserID` matches authenticated user.

---

### Download file

```
GET /{id}
```

Serves the file with appropriate `Content-Disposition`:
- `inline` for video/, image/, application/pdf types
- `attachment` for all other types

---

### Rename file

```
PATCH /{id}/rename
```

**Body:** `{ "filename": "new-name.mp4" }`

**Success response:** `{ "filename": "new-name.mp4" }`

---

### Move file to folder

```
PATCH /{id}/move
```

**Body:** `{ "folder": "/photos" }`

**Success response:** `{ "folder": "/photos" }`

---

### Delete file (soft delete to trash)

```
DELETE /{id}
```

**Success response:** `{ "status": "trashed" }`

---

### Batch delete

```
POST /batch/delete
```

**Body:** `{ "items": [{"kind": "file", "id": "uuid"}, ...] }`

**Success response:** `{ "status": "trashed" }`

---

### Batch move

```
PATCH /batch/move
```

**Body:** `{ "items": [{"kind": "file", "id": "uuid"}, ...], "folder": "/photos" }`

**Success response:** `{ "folder": "/photos" }`

---

### Batch download as ZIP

```
POST /batch/download
```

**Body:** `{ "items": [{"kind": "file", "id": "uuid"}, ...] }`

**Headers:** `Content-Type: application/zip`, `Content-Disposition: attachment; filename="media-download.zip"`

---

### Thumbnail generation

```
GET /{id}/thumb
```

Returns the file's thumbnail as JPEG with `Cache-Control: public, max-age=31536000, immutable`.

---

### Recent uploads and playback

```
GET /recent
```

**Query parameters:** None.

**Response:**

```json
{
  "recent_uploads": [...],
  "recently_played": [...]
}
```

---

### Storage statistics

```
GET /stats
```

**Response:**

```json
{
  "total_size": 1234567890,
  "total_size_formatted": "1.2 GB",
  "file_count": 42,
  "categories": {
    "video": 15,
    "image": 20,
    "document": 5,
    "other": 2
  }
}
```

---

### File status endpoints

```
PATCH /{id}/progress
```

**Body:** `{ "progress": 45 }` (0-100)

```
PATCH /{id}/rename   (renames the folder, not the file)
PATCH /{id}/move     (moves file to new folder)
```

---

## Folder Endpoints

### Create folder

```
POST /mkdir
```

**Body:** `{ "path": "/photos/vacation" }`

**Success response:** `{ "path": "/photos/vacation" }`

---

### Rename folder

```
PATCH /folder/{id}/rename
```

**Body:** `{ "name": "new-name" }`

**Success response:** `{ "path": "/new-name" }`

---

### List user folders

```
GET / (lists folders alongside files, see ListHandler)
```

Or query folder structure explicitly through the database layer.

---

## Job Queue Endpoints

### Get job status

```
GET /jobs/{id}
```

**URL parameters:**

| Parameter | Description |
|---|---|
| `id` | Job UUID |

**Response:** Job status and progress.

---

## Error Handling

All errors return JSON with at minimum:

```json
{ "error": "error message" }
```

HTTP status codes:

| Code | Meaning |
|---|---|
| 400 | Bad request (validation, invalid ID, missing fields) |
| 401 | Unauthorized (missing/invalid auth) |
| 403 | Forbidden (user tries to access another user's resource) |
| 404 | Not found (file/folder doesn't exist) |
| 413 | Payload too large (upload exceeds max size) |
| 500 | Internal server error |

Error responses from `utils.RespondWithError` follow this pattern.