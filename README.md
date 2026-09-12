# Project mediaserver

MediaServer is a self-hosted media library. For a portable Docker deployment on
an external SSD, use the included `mediaserver` CLI. It stores the SQLite
database and uploaded media together on the configured SSD path.

## SSD Docker deployment

The default project location is `/mnt/mediaserver-ssd/mediaserver` and the
default persistent-data directory is `data/` inside it. Install the manager
from the project root, then run the one-time setup:

```bash
go install ./cmd/mediaserver
mediaserver setup
```

Use another location or port during setup when needed:

```bash
mediaserver setup --project-dir /mnt/mediaserver-ssd/mediaserver \
  --data-path /mnt/mediaserver-ssd/media-data --port 8080
```

The CLI saves non-secret deployment settings in
`~/.config/mediaserver/config.json`. Application secrets remain in `.env`.

```bash
mediaserver start
mediaserver stop
mediaserver status
mediaserver update                 # clean tree only: pull, rebuild, restart
mediaserver config get
mediaserver config set port 9090
mediaserver config set data-path /mnt/mediaserver-ssd/media-data
mediaserver autostart enable
mediaserver autostart disable
mediaserver remove                 # keeps database and uploads
mediaserver remove --purge-data --force
```

`setup` installs a user systemd service that waits for both the SSD project
and data mounts. Back up the configured data directory, especially `db/` and
`storage/`, before moving or deleting the drive. `dockdb` is not used here:
MediaServer uses SQLite, while DockDB manages PostgreSQL and MySQL containers.

## Getting Started

These instructions will get you a copy of the project up and running on your local machine for development and testing purposes. See deployment for notes on how to deploy the project on a live system.

## MakeFile

Run build make command with tests
```bash
make all
```

Build the application
```bash
make build
```

Run the application
```bash
make run
```
Create DB container
```bash
make docker-run
```

Shutdown DB Container
```bash
make docker-down
```

DB Integrations Test:
```bash
make itest
```

Live reload the application:
```bash
make watch
```

Run the test suite:
```bash
make test
```

Clean up binary from the last build:
```bash
make clean
```
