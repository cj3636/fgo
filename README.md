# fGo — Git-Lite File Server

[![pipeline status](https://git.tyss.io/cj3636/fgo/badges/main/pipeline.svg?style=for-the-badge)](https://git.tyss.io/cj3636/fgo/-/pipelines)
[![coverage report](https://git.tyss.io/cj3636/fgo/badges/main/coverage.svg?style=for-the-badge)](https://git.tyss.io/cj3636/fgo/-/commits/main)
[![Go Version](https://img.shields.io/badge/go-1.24-blue?style=for-the-badge)](https://golang.org/doc/go1.24)
[![License](https://img.shields.io/badge/license-MIT-green?style=for-the-badge)](LICENSE)

A simple, fast, and secure Git-like file server for personal and small team use. Implements linear versioning, idempotent blob uploads, and minimal API surface. Designed for homelab and production-grade deployment.

---

## Quickstart (End Users)

### Prerequisites

- Go 1.24+
- SQLite (no CGO required)
- Linux, Windows, or MacOS

### Run the Server

```sh
git clone https://git.tyss.io/cj3636/fgo.git
cd fgo
go build -o gofile ./cmd/gofile
./gofile
```

Server runs on `:8080` by default.

### Basic API Usage

- Health: `GET /v0/health`
- List boxes: `GET /v0/boxes`
- Create box: `POST /v0/boxes` (JSON: `{name, visibility, default_branch}`)
- Plan push: `POST /v0/boxes/<box>/push/plan`
- Upload blob: `PUT /v0/blobs/<sha256>`
- Finalize push: `POST /v0/boxes/<box>/push/finalize`
- Latest commit: `GET /v0/boxes/<box>/commits/latest?branch=main`
- Download file: `GET /v0/files/<commit_id>?path=<file>`
- OpenAPI spec: `GET /v0/openapi.yaml`

See [openapi.yaml](openapi.yaml) for full API details.

---

## Developer Quickstart

### Build & Test

```sh
go build ./...
go test ./...
```

### Directory Structure

```
cmd/gofile/      # Server main
cmd/fgo/         # CLI client (future)
internal/        # Core packages
  storage/       # BlobStore, MetaStore
  auth/          # Auth interfaces
  httpx/         # Routing/middleware
  domain/        # Services
  integrity/     # Checksums, hooks
  observe/       # Logging, metrics
```

### Run Locally

```sh
go run ./cmd/gofile
```

### API Smoke Test

```sh
curl -s http://localhost:8080/v0/health
```

---

## CI/CD & Coverage

- All pushes trigger pipeline and coverage jobs on GitLab CI
- Coverage badge reflects latest main branch
- Container builds use [registry.tyss.io/cj3636/ci-images/go1.24:latest](https://registry.tyss.io/cj3636/ci-images/go1.24:latest)

---

## Contributing

- Fork and submit merge requests via [GitLab](https://git.tyss.io/cj3636/fgo)
- All code must pass tests and coverage
- See [copilot-instructions.md](copilot-instructions.md) for roadmap and design

---

## License

MIT
