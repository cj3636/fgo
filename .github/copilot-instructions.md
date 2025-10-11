# fGo — Alpha Project Roadmap (Git‑Lite File Server)

> Repo: `git.tyss.io/cj3636/fgo`
> Go: **1.24**
> Purpose: A simple, fast, and secure VCS File Server with Git-like versioning for personal and small team use.
> Deployment: homelab first, production-grade patterns included

- ***NOTE:*** This document uses fgo ⇄ git command names interchangeably.
  - The application code and documentation, and user interface should ONLY use the custom fgo terminology.
  - git terminology is only for user familiarity in help text and CLI aliases.

---

## Overview

A pragmatic, extensible roadmap that ships working software at every milestone. Focus is the **server first**, with a
minimal but stable wire protocol and a thin CLI later. Each stage is production‑usable within its stated limits.

**Design priorities:** speed → data integrity → security → simplicity. Everything is pluggable behind small Go interfaces.

---

## Naming

- `fgo` CLI client (Git‑lite; like `git`)
- `gofile` server (HTTP API)

---

## 0) Canon & Terminology

**Canonical (API / server) names**:

- **space** (top‑level partition)
- **box** (a versioned collection; like a repo)
- **branch** (mutable ref to a commit, default `main`)
- **commit** (immutable manifest of paths → blobs)
- **blob** (content‑addressed file by sha256)
- **visibility**: `public | unlisted | private`

*Display and support the git command name aliases in the CLI for familiarity.*

**Client Commands** — exposed by `fgo` :

- space ⇄ **namespace**
- box ⇄ **repo**
- arm ⇄ **branch**
- enact ⇄ **commit**
- dupe ⇄ **clone**
- grab ⇄ **pull**
- call ⇄ **fetch**
- place ⇄ **push**
- replace ⇄ **force‑push to branch head**
- store ⇄ **push to specific arm**

> Rule: protocol & API use these canonical terms. Help text maps my names ↔ canonical names *only* for user convenience.

**IDs:** ULID (sortable) for boxes/commits; blobs by sha256. JSON fields use `snake_case`.

---

## 1) Architecture & Packages (pluggable by interface)

```bash
internal/
  httpx/           # routing, middleware, headers, authn, errors
  auth/            # token & basic auth; interfaces
  domain/          # services: files, boxes, versioning, visibility
  storage/
    blobstore/     # BlobStore (fs, s3, …)
    metastore/     # MetadataStore (sqlite, pg)
  integrity/       # checksums, MIME, antivirus hooks (opt)
  pgp/             # optional detached signing/verify (v2+)
  observe/         # logs, metrics, tracing
cmd/
  gofile/          # server main
  fgo/             # CLI client (after server MVP)
```

**Core interfaces** (stable early):

```go
// storage/blobstore.go
type BlobStore interface {
Has(ctx context.Context, sha string) (bool, error)
Put(ctx context.Context, sha string, r io.Reader, size int64) error
Open(ctx context.Context, sha string) (io.ReadCloser, int64, error)
}

// storage/metastore.go
type MetadataStore interface {
CreateBox(ctx context.Context, b Box) (Box, error)
GetBox(ctx context.Context, ns, name string) (Box, error)
SaveCommit(ctx context.Context, c Commit) (Commit, error)
LatestCommit(ctx context.Context, boxID string, branch string) (Commit, error)
MoveRef(ctx context.Context, boxID, branch, parentID, newID string) error // optimistic lock
ListPublicBoxes(ctx context.Context) ([]Box, error)
}

// auth/auth.go
type Authenticator interface {
Authenticate(r *http.Request) (Principal, error)
}
```

**Data model (MVP):**

```json
// Box
{
  "id": "01J...",
  "namespace_id": "01K...",
  "name": "gofiles",
  "visibility": "public",
  "default_branch": "main"
}

// Commit (manifest)
{
  "id": "01J...",
  "box_id": "01J...",
  "branch": "main",
  "parent_id": null,
  "message": "init",
  "author": "token:abc…",
  "timestamp": "…",
  "entries": [
    {
      "path": "README.md",
      "sha256": "…",
      "size": 1234,
      "mode": 420
    }
  ]
}
```

---

## 2) API Surface (MVP‑first; idempotent)

- /v0 -> versioned API prefix. Breaking changes allowed without documentation until v1. **No** backwards compatibility or migrations until an v1.

**System**

- `GET /v0/health`
- `GET /v0/openapi.json`  •  `GET /v0/docs`

**Blobs (idempotent)**

- `HEAD /v0/blobs/{sha256}` → 200 if present
- `PUT /v0/blobs/{sha256}` with `Digest: sha-256=…`, `Content-Length`

**Push (3‑step)**

1. **Plan**: `POST /v0/boxes/{box}/push/plan` body `{ entries:[{path,sha256,size}] }`
   ← `{ missing:[sha256…], total:N, will_replace:M }`
2. **Upload**: PUT missing blobs.
3. **Finalize**: `POST /v0/boxes/{box}/push/finalize` `{ branch, parent_commit_id, message, entries:[…] }`
   ← `{ commit_id, uploaded:K, reused:M }`  (409 if parent mismatch)

**Pull / Browse**

- `GET /v0/boxes?visibility=public`  (public listing)
- `GET /v0/boxes/{box}/commits/latest?branch=main`
- `GET /v0/boxes/{box}/tree/{commit_id}` → file listing
- `GET /v0/files/{commit_id}/{path}` (supports `Range`, `ETag`)

**Boxes / Spaces**

- `POST /v0/spaces` / `GET /v0/spaces/{space}`
- `POST /v0/boxes` / `GET /v0/boxes/{box}` / `PATCH /v0/boxes/{box}` (visibility)
- `DELETE /v0/boxes/{box}` → tombstone w/ `expires_at`; `POST /restore` (within grace period)

**Auth (alpha)**

- HTTP Basic (for admin bootstrap), **Bearer token** for normal use. Tokens stored hashed (Argon2id).

**Headers & Caching**

- Upload: `Digest: sha-256=base64`, `If-Match` for finalize (parent commit).
- Download: `ETag`, `Cache-Control`, `Range`.

---

## 3) Milestones (each yields a working app)

### Milestone 1 — **Public files + rudimentary versioning** (v0.1)

**Goal:** A usable server that serves public files, linear history on `main`, simple auth.

**Scope**

- Router & middleware: logging, panic guard, CORS, gzip
- Auth: HTTP Basic (bootstrap), Bearer token (hash at rest)
- BlobStore FS; MetadataStore SQLite
- Endpoints: health, openapi/docs, blobs, push.plan/finalize, latest commit, file GET
- Boxes: create, get, list (public only); `visibility` respected for reads
- Versioning: linear commits on `main` only, optimistic finalize (parent check)
- Observability: JSON logs; counters for put/get/plan/finalize

**Deliverables**

- `gofile` server binary
- Minimal OpenAPI
- Smoke CLI via curl; tiny helper script (`fgo` optional stub)

**Tests**

- Unit: sha256, blob put/open, plan diff, finalize transaction
- HTTP: upload cycle (plan→put→finalize), download w/ ETag & Range
- Concurrency: two finalize calls → one 409

**Risks/notes**

- No branches, no ACL beyond visibility, no unlisted tokens yet
- Manual GC only (no background sweeps)

---

### Milestone 2 — **History & Listings + CLI MVP** (v0.2)

**Scope**

- `GET /tree/{commit_id}` and `GET /commits/{branch}` (recent N)
- Archive export: `GET /boxes/{box}/download?format=zip|tar.gz` (latest only)
- Integrity: enforce SHA‑256 match on finalize (size+digest)
- Basic Web UI: `/browse` (plain index), `/upload` form
- `fgo` MVP: `init|add|commit|push|pull|status` acting on a local manifest

**Tests**

- E2E: create → add files → commit → push → pull → compare digests
- Index/listing stability; archive contents match tree

---

### Milestone 3 — **Branches & Conflicts, Visibility polish** (v0.3)

**Scope**

- Branch support: create/list/switch/delete; default `main`
- Finalize requires `parent_commit_id`; 409 on divergence
- Visibility behavior: `public|unlisted|private` (server hides unlisted from list)
- Signed download URLs for **unlisted** (HMAC token with `{box, path, commit, exp}`)
- Observability: Prometheus metrics; request/size histograms

**CLI**

- `arm` (alias: `branch`) subcommands; `push --branch dev`
- `--dry-run` for uploads (show plan)

**Tests**

- Divergent finalize path; signed URL expiry; visibility leak checks

---

### Milestone 4 — **Tokens, RBAC-lite, Tombstones & GC** (v0.4)

**Scope**

- Namespaces (ownership boundary); tokens are bound to namespace with scopes: `read`, `write`, `admin`
- Delete → tombstone with `expires_at`; restore API within grace period; GC job removes blobs unreferenced by any commit
- Rate limiting; request IDs; audit logging (who pushed what)

**CLI**

- Profiles (`~/.fgo/config.json`): base URL, token per namespace
- `fgo log` commit history; `fgo show` entry metadata

**Tests**

- GC safety: referenced blobs preserved; restore works until grace expires

---

### Milestone 5 — **Large files & Batch UX** (v0.5)

**Scope**

- Resumable uploads (TUS‑style or S3 multipart analogue) behind `BlobStore` interface; fall back to single‑PUT
- Parallelism controls (`?part=N` or `--parallel` on CLI)
- Batch grammar: `batch init/open/ls/show/rm`, `add|rm|commit|push --dry-run`, `export|import`

**Tests**

- Kill/restart during multipart; resume works; server idempotent
- Batch import/export round‑trip reproducibility

---

### Milestone 6 — **Pluggable Backends & Hooks** (v0.6)

**Scope**

- Postgres `MetadataStore`
- S3/GCS `BlobStore`
- Webhooks (post‑finalize): notify external systems
- Optional PGP: detached signature/verify for manifests

**Tests**

- Cross‑store parity; migration from SQLite→Postgres

---

### Milestone 7 — **RC hardening & docs** (v1.0‑rc)

**Scope**

- Threat model & security review; fuzzing on path parsing and manifest input
- Load/perf baselines (concurrency, FS vs S3)
- Backwards‑compat boundary set; schema migration plan v1→v1.x
- Full docs: OpenAPI examples, CLI manpage, admin guide

---

## 4) Implementation Details & Patterns

### Push pipeline (server)

```go
// PLAN: compute missing blobs once per manifest
func PlanMissing(blobs BlobStore, entries []Entry) ([]string, error) {
seen := map[string]struct{}{}
var missing []string
for _, e := range entries {
if _, ok := seen[e.SHA256]; ok { continue }
seen[e.SHA256] = struct{}{}
ok, err := blobs.Has(ctx, e.SHA256)
if err != nil { return nil, err }
if !ok { missing = append(missing, e.SHA256) }
}
sort.Strings(missing)
return missing, nil
}

// FINALIZE: verify all blobs exist, validate sizes/digests, write commit, move ref atomically.
err := meta.Tx(ctx, func(tx MetadataStore) error {
for _, e := range entries { if !blobs.Has(ctx, e.SHA256) { return ErrMissingBlob } }
newC := Commit{ParentID: parent, Branch: branch, Entries: entries, Message: msg}
newC = meta.SaveCommit(ctx, newC)
return meta.MoveRef(ctx, boxID, branch, parent, newC.ID) // fails if ref != parent
})
```

### Download pipeline

- Resolve `{box, branch}` → `commit_id`
- Stream file by `sha256` from `BlobStore.Open`
- Set `ETag: W/"sha256:…"`, honor `Range`

### Visibility & unlisted

- `public`: visible in lists
- `unlisted`: never in lists; access by signed URL token
- `private`: token‑gated

### Naming & validation

- POSIX paths in manifests; reject `..`, control chars, NUL
- Branch names: `^[A-Za-z0-9._/-]{1,64}$` (reserve `main`)
- Box names: kebab‑case; namespaces lowercase DNS‑ish

### Error codes

- 400 invalid manifest / names
- 401 unauthenticated / 403 unauthorized
- 404 box/commit/path not found
- 409 parent mismatch on finalize
- 422 digest/size mismatch
- 429 rate limit

### Observability

- Structured JSON logs with request\_id
- Prometheus counters: `http_requests_total`, `blob_put_bytes_total`, `push_finalize_total{status=…}`
- Latency histograms for blob PUT, finalize, file GET

---

## 5) Database Sketch (SQLite → Postgres)

```sql
CREATE TABLE boxes (
  id TEXT PRIMARY KEY,
  namespace_id TEXT NOT NULL,
  name TEXT NOT NULL,
  visibility TEXT NOT NULL CHECK (visibility IN ('public','unlisted','private')),
  default_branch TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(namespace_id, name)
);

CREATE TABLE commits (
  id TEXT PRIMARY KEY,
  box_id TEXT NOT NULL,
  branch TEXT NOT NULL,
  parent_id TEXT,
  message TEXT,
  author TEXT,
  timestamp TEXT NOT NULL,
  FOREIGN KEY(box_id) REFERENCES boxes(id)
);

CREATE TABLE entries (
  commit_id TEXT NOT NULL,
  path TEXT NOT NULL,
  sha256 TEXT NOT NULL,
  size INTEGER NOT NULL,
  mode INTEGER NOT NULL,
  PRIMARY KEY(commit_id, path),
  FOREIGN KEY(commit_id) REFERENCES commits(id)
);

CREATE TABLE refs (
  box_id TEXT NOT NULL,
  branch TEXT NOT NULL,
  commit_id TEXT NOT NULL,
  PRIMARY KEY(box_id, branch)
);

CREATE TABLE tokens (
  id TEXT PRIMARY KEY,
  namespace_id TEXT NOT NULL,
  name TEXT NOT NULL,
  hash TEXT NOT NULL, -- Argon2id
  scope TEXT NOT NULL, -- read|write|admin
  created_at TEXT NOT NULL,
  revoked_at TEXT
);
```

GC: sweep blobs where `sha256` not present in any `entries` of any live commit and not in tombstoned grace window.

---

## 6) CLI (`fgo`) Grammar (aliases included)

**Current batch context** in `~/.fgo/current_batch`. Git‑like top‑level commands operate on it.

```
fgo init <box> [--namespace ns]              # alias: box create
fgo clone <ns>/<box> [dir]                   # alias: dupe
fgo add <path…> | rm <path…>
fgo commit -m "msg"                           # alias: enact
fgo push [--branch main] [--dry-run]         # alias: place
fgo pull [--branch main]                      # alias: grab
fgo branch create|list|switch|delete …        # alias: arm
fgo status | log | show <path>

# Batch helpers
fgo batch init|open|ls|show|rm
fgo export <file.json> | import <file.json>
```

Conventions: `--json` for scriptable output; deterministic exit codes (0 ok, 3 no‑op, 409 conflict).

---

## 7) Security & Caveats

- Start with **one‑token auth** (Bearer); keep HTTP Basic only for initial admin.
- Hash tokens with Argon2id; rotate easily.
- All endpoints behind TLS.
- Input hardening: reject path traversal; enforce max path len & file size (configurable).
- Unlisted links via signed tokens; never show unlisted in list APIs.
- Tombstones + grace: safer deletes; GC after window.

Known limitations before v1:

- No resumable uploads until Milestone 5.
- No RBAC beyond namespace‑level scopes until Milestone 4.

---

## 8) Ops Checklist per Milestone

- Systemd unit for `gofile`; health probe
- Config file with `blob_store`, `meta_store`, limits
- Log rotation; metrics endpoint `/metrics` (from M3)
- Backup: SQLite WAL snapshot; blob dir rsync/s3 sync

---

## 9) Decision Log (template)

[Copilot Log File](../.github/copilot.log.md)

Log consequential choices; keep short.

```
- 2025‑08‑21 Use ULID for ids — sortable & collision‑safe.
- 2025‑08‑21 Manifest entries use POSIX paths — cross‑platform.
- 2025‑08‑21 Plan/finalize split — idempotent uploads, resumable later.
```

---

## 10) Next Actions (to start M1 now)

1. Wire `httpx` with middlewares (recover, request\_id, logging, CORS, gzip)
2. Implement `BlobStoreFS` and `SQLiteMetaStore`
3. Implement `POST /boxes`, `GET /boxes`, `GET /v0/health`
4. Implement **plan → put blobs → finalize**
5. Implement `GET /files/{commit}/{path}` with `ETag` and `Range`
6. Generate OpenAPI from handlers; publish `/docs`
7. Write 8–10 high‑value tests (plan/finalize conflict, digest mismatch, range get)

> Ship v0.1 with a caution banner: linear history only, limited auth, manual GC.
