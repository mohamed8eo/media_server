# Web UI Documentation

MediaServer's frontend is built with [templ](https://github.com/a-h/templ),
HTMX, [AlpineJS], and [TailwindCSS](https://tailwindcss.com) via CDN.

## Architecture

### Component hierarchy

```
base.templ          # Root layout with HTMX attributes
├── sidebar.templ   # Collapsible navigation menu
├── home.templ      # Dashboard / landing page
├── all_media.templ # Full media library view
├── upload.templ    # File upload page
├── settings.templ  # Configuration page
├── watch.templ     # Job monitoring page
├── reader.templ    # Document & PDF reader view
└── shared/
    ├── file_card.templ   # Individual file display component
    └── folder_card.templ # Folder display component
```

### Key templ components

| Component | Purpose |
|---|---|
| `base_templ.go` + `base.templ` | HTML5 layout, HTMX attributes (`hx-on`, `hx-trigger`), AlpineJS initialization |
| `sidebar_templ.go` + `sidebar.templ` | Navigation with collapsible state, active route highlighting |
| `home_templ.go` + `home.templ` | Dashboard with storage stats, recent uploads, quick actions |
| `all_media_templ.go` + `all_media.templ` | Full library grid view with filtering and search |
| `upload_templ.go` + `upload.templ` | Drag-and-drop upload zone, folder selector |
| `file_card_templ.go` + `file_card.templ` | Reusable card displaying file metadata, actions |
| `folder_card_templ.go` + `folder_card.templ` | Folder visualization with nested children |
| `recent_section_templ.go` + `recent_section.templ` | Section showing recent uploads/playback |
| `settings_templ.go` + `settings.templ` | User configuration form |
| `watch_templ.go` + `watch.templ` | Job queue monitoring and control |
| `reader_templ.go` + `reader.templ` | Dedicated document & PDF reader with progress tracking and mobile-optimized scaling |

### HTMX integration

All UI updates use HTMX triggers rather than full page reloads:

| Trigger | Listener | Purpose |
|---|---|---|
| `mediaUpdated` | `.media-list` | Refresh file grid after upload/delete/move |
| `folderUpdated` | `.folder-list` | Refresh folder navigation |
| `statusUpdated` | `.job-item` | Update job progress status |
| `trashUpdated` | `.trash-section` | Refresh recycle bin contents |
| `emptyTrashComplete` | `.empty-trash-btn` | Confirm trash emptying |

### AlpineJS usage

Global Alpine store (`app_shell.templ`) provides:

- `sidebarOpen` — toggles mobile sidebar
- `searchQuery` — filters media list
- `selectedItems` — tracks batch operation selections
- `confirmModal` — manages confirmation dialogs

### TailwindCSS

Utility-first CSS via CDN:
```
https://cdn.tailwindcss.com
```

Plus AlpineJS:
```
https://cdn.jsdelivr.net/npm/alpinejs
```

Custom styles are minimal — most styling comes from Tailwind utility classes.

### Static assets

| Path | Description |
|---|---|
| `cmd/web/assets/` | Icons, favicon, sw.js (service worker) |
| `cmd/web/app_shell.templ` | Root HTML template with AlpineJS init |
| `cmd/web/*.templ` | All UI pages and components |

### Page flow

1. **Home** (`/`) — Dashboard with storage stats, recent uploads, quick actions
2. **Media library** (`/all-media`) — Grid view of all files with folders sidebar
3. **Upload** (`/upload`) — File upload with folder selection and yt-dlp URL paste
4. **Settings** (`/settings`) — JWT, storage path, environment config
5. **Job watch** (`/watch`) — Background job status and controls
6. **Document Reader** (`/reader/{id}`) — Custom PDF reader with continuous scroll, rotation, page navigation, and playback progress syncing

### Responsive behavior

- Desktop: Full sidebar + grid layout
- Tablet: Collapsible sidebar, adjusted grid columns
- Mobile: Bottom sheet sidebar, single-column grid, and optimized narrow-padding view for PDF reader to maximize screen utilization without manual zooming.

### Customization

To add a new page:

1. Create `*.templ` and `*_templ.go` in `cmd/web/`
2. Add route registration in `internal/server/routes.go` if needed
3. Add HTMX triggers and AlpineJS state as required
4. Update `base.templ` if new global elements are needed

---

Need help with a specific component? See the source files in `cmd/web/` or the
[API reference](./docs/api-reference.md) for endpoint details.