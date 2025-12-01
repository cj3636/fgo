package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"fgo/internal/httpx"
	"fgo/internal/storage/blobstore"
	"fgo/internal/storage/metastore"
)

func main() {
	fmt.Println("gofile server starting...")

	// TODO: Implement real authentication
	// Placeholder for Authenticator interface. Wire up real implementation in future milestones.
	// var authenticator auth.Authenticator

	// Load config.yaml
	type Config struct {
		Port      int    `yaml:"port"`
		BlobStore string `yaml:"blob_store"`
		MetaStore string `yaml:"meta_store"`
	}
	var cfg Config
	f, err := os.Open("config.yaml")
	if err != nil {
		log.Fatalf("failed to open config.yaml: %v", err)
	}
	defer f.Close()
	dec := yaml.NewDecoder(f)
	if err := dec.Decode(&cfg); err != nil {
		log.Fatalf("failed to parse config.yaml: %v", err)
	}
	fmt.Printf("[CONFIG] port=%d blob_store=%s meta_store=%s\n", cfg.Port, cfg.BlobStore, cfg.MetaStore)

	// Initialize BlobStoreFS and SQLiteMetaStore
	_ = os.MkdirAll(cfg.BlobStore, 0755)
	blobs := blobstore.NewBlobStoreFS(cfg.BlobStore)
	meta, err := metastore.NewSQLiteMetaStore(cfg.MetaStore)
	if err != nil {
		log.Fatalf("failed to open metastore: %v", err)
	}

	mux := http.NewServeMux()

	// Basic Web UI: /browse (public boxes)
	mux.HandleFunc("/browse", func(w http.ResponseWriter, r *http.Request) {
		boxes, err := meta.ListPublicBoxes(r.Context())
		if err != nil {
			http.Error(w, "error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "<html><head><title>fGo Browse</title></head><body><h1>Public Boxes</h1><ul>")
		for _, b := range boxes {
			fmt.Fprintf(w, "<li><a href='/browse/%s'>%s</a></li>", b.Name, b.Name)
		}
		fmt.Fprintf(w, "</ul></body></html>")
	})

	// Basic Web UI: /upload (simple form)
	mux.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `<html><head><title>fGo Upload</title></head><body><h1>Upload File</h1><form method='POST' enctype='multipart/form-data'><input type='file' name='file'><input type='submit'></form></body></html>`)
			return
		}
		if r.Method == http.MethodPost {
			f, h, err := r.FormFile("file")
			if err != nil {
				http.Error(w, "upload error", http.StatusBadRequest)
				return
			}
			defer f.Close()
			// For demo: just show file name and size
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "<html><body>Uploaded: %s (%d bytes)</body></html>", h.Filename, h.Size)
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	})

	// Health
	mux.HandleFunc("/v0/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Boxes list/create only at exact /v0/boxes (no trailing slash)
	mux.HandleFunc("/v0/boxes", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/boxes" {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			boxes, err := meta.ListPublicBoxes(r.Context())
			if err != nil {
				http.Error(w, "error", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(boxes)
		case http.MethodPost:
			var req struct {
				Name          string `json:"name"`
				Visibility    string `json:"visibility"`
				DefaultBranch string `json:"default_branch"`
			}
			rawBody, _ := io.ReadAll(r.Body)
			fmt.Printf("[DEBUG] finalize raw body: %s\n", string(rawBody))
			os.Stdout.Sync()
			if err := json.Unmarshal(rawBody, &req); err != nil {
				http.Error(w, "bad json", http.StatusBadRequest)
				return
			}
			// Only print ParentCommitID in finalize handler, not boxes handler
			if req.Name == "" {
				http.Error(w, "name required", http.StatusBadRequest)
				return
			}
			if req.DefaultBranch == "" {
				req.DefaultBranch = "main"
			}
			b := metastore.Box{NamespaceID: "global", Name: req.Name, Visibility: req.Visibility, DefaultBranch: req.DefaultBranch}
			b, err = meta.CreateBox(r.Context(), b)
			if err != nil {
				http.Error(w, "error", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(b)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Consolidated router for /v0/boxes/{box}/...
	mux.HandleFunc("/v0/boxes/", func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/v0/boxes/")
		parts := strings.Split(p, "/")
		boxName := parts[0]
		action := ""
		if len(parts) > 1 {
			action = strings.Join(parts[1:], "/")
		}
		box, err := meta.GetBox(r.Context(), "global", boxName)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(action, "tree/"):
			// GET /v0/boxes/{box}/tree/{commit_id}
			parts := strings.Split(action, "/")
			if len(parts) != 2 || parts[0] != "tree" {
				http.NotFound(w, r)
				return
			}
			commitID := parts[1]
			commit, err := meta.GetCommitByID(r.Context(), commitID)
			if err != nil {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			// Return file listing (entries)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(commit.Entries)
		case r.Method == http.MethodGet && action == "":
			// GET /v0/boxes/{box}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(box)
		case r.Method == http.MethodGet && strings.HasPrefix(action, "commits"):
			// GET /v0/boxes/{box}/commits?branch=main&limit=N
			branch := r.URL.Query().Get("branch")
			if branch == "" {
				branch = box.DefaultBranch
			}
			limit := 10
			if l := r.URL.Query().Get("limit"); l != "" {
				if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
					limit = n
				}
			}
			commits, err := meta.ListCommits(r.Context(), box.ID, branch, limit)
			if err != nil {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(commits)
		case r.Method == http.MethodPost && action == "push/plan":
			var req struct {
				Entries []metastore.Entry `json:"entries"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "bad json", http.StatusBadRequest)
				return
			}
			seen := map[string]struct{}{}
			missing := []string{}
			for _, e := range req.Entries {
				if _, ok := seen[e.SHA256]; ok {
					continue
				}
				seen[e.SHA256] = struct{}{}
				ok, err := blobs.Has(r.Context(), e.SHA256)
				if err != nil {
					http.Error(w, "error", http.StatusInternalServerError)
					return
				}
				if !ok {
					missing = append(missing, e.SHA256)
				}
			}
			resp := map[string]any{"missing": missing, "total": len(req.Entries), "will_replace": 0}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)

		case r.Method == http.MethodPost && action == "push/finalize":
			var req struct {
				Branch         string            `json:"branch"`
				ParentCommitID string            `json:"parent_commit_id"`
				Message        string            `json:"message"`
				Entries        []metastore.Entry `json:"entries"`
			}
			rawBody, _ := io.ReadAll(r.Body)
			fmt.Printf("[DEBUG] finalize raw body: %s\n", string(rawBody))
			if err := json.Unmarshal(rawBody, &req); err != nil {
				http.Error(w, "bad json", http.StatusBadRequest)
				return
			}
			fmt.Printf("[DEBUG] finalize decoded parent_commit_id: '%s'\n", req.ParentCommitID)
			os.Stdout.Sync()
			if req.Branch == "" {
				req.Branch = box.DefaultBranch
			}
			for _, e := range req.Entries {
				ok, err := blobs.Has(r.Context(), e.SHA256)
				if err != nil {
					http.Error(w, "error", http.StatusInternalServerError)
					return
				}
				if !ok {
					http.Error(w, "missing blob", http.StatusUnprocessableEntity)
					return
				}
			}

			var parentPtr *string
			if req.ParentCommitID != "" {
				parentPtr = &req.ParentCommitID
			}
			fmt.Printf("[DEBUG] finalize: box=%s branch=%s parentID='%s' parentPtr=%v\n", box.ID, req.Branch, req.ParentCommitID, parentPtr)
			commit := metastore.Commit{BoxID: box.ID, Branch: req.Branch, ParentID: parentPtr, Message: req.Message, Author: "", Entries: req.Entries}
			commit, err = meta.SaveCommit(r.Context(), commit)
			if err != nil {
				http.Error(w, "error", http.StatusInternalServerError)
				return
			}
			fmt.Printf("[DEBUG] finalize: box=%s branch=%s parentPtr=%v newID=%s\n", box.ID, req.Branch, parentPtr, commit.ID)
			parentID := ""
			if parentPtr != nil {
				parentID = *parentPtr
			}
			if err := meta.MoveRef(r.Context(), box.ID, req.Branch, parentID, commit.ID); err != nil {
				fmt.Printf("[DEBUG] MoveRef error: %v\n", err)
				if strings.Contains(err.Error(), "parent mismatch") {
					http.Error(w, "parent mismatch", http.StatusConflict)
					return
				}
				http.Error(w, "error", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"commit_id": commit.ID, "uploaded": 0, "reused": 0})

		case r.Method == http.MethodGet && action == "commits/latest":
			branch := r.URL.Query().Get("branch")
			if branch == "" {
				branch = "main"
			}
			commit, err := meta.LatestCommit(r.Context(), box.ID, branch)
			if err != nil {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(commit)

		default:
			http.NotFound(w, r)
		}
	})

	// Blobs: HEAD/PUT /v0/blobs/{sha256}
	mux.HandleFunc("/v0/blobs/", func(w http.ResponseWriter, r *http.Request) {
		sha := strings.TrimPrefix(r.URL.Path, "/v0/blobs/")
		if sha == "" {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodHead:
			ok, err := blobs.Has(r.Context(), sha)
			if err != nil {
				http.Error(w, "error", http.StatusInternalServerError)
				return
			}
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.WriteHeader(http.StatusOK)
		case http.MethodPut:
			size := r.ContentLength
			if size < 0 {
				http.Error(w, "length required", http.StatusLengthRequired)
				return
			}
			if err := blobs.Put(r.Context(), sha, r.Body, size); err != nil {
				http.Error(w, "error", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusCreated)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Files: GET /v0/files/{commit_id}?path=... with Range support
	mux.HandleFunc("/v0/files/", func(w http.ResponseWriter, r *http.Request) {
		commitID := strings.TrimPrefix(r.URL.Path, "/v0/files/")
		p := r.URL.Query().Get("path")
		if commitID == "" || p == "" {
			http.Error(w, "missing", http.StatusBadRequest)
			return
		}
		commit, err := meta.GetCommitByID(r.Context(), commitID)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		var entry *metastore.Entry
		for _, e := range commit.Entries {
			if e.Path == p {
				entry = &e
				break
			}
		}
		if entry == nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		rc, size, err := blobs.Open(r.Context(), entry.SHA256)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		defer rc.Close()
		etag := "W/\"sha256:" + entry.SHA256 + "\""
		w.Header().Set("ETag", etag)
		if inm := r.Header.Get("If-None-Match"); inm != "" && inm == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		// Handle Range header: bytes=start-end or bytes=start-
		rangeHdr := r.Header.Get("Range")
		if rangeHdr != "" && strings.HasPrefix(rangeHdr, "bytes=") {
			rng := strings.TrimPrefix(rangeHdr, "bytes=")
			var start, end int64
			end = size - 1
			if strings.Contains(rng, "-") {
				parts := strings.SplitN(rng, "-", 2)
				if parts[0] != "" {
					s, _ := strconv.ParseInt(parts[0], 10, 64)
					start = s
				}
				if parts[1] != "" {
					e, _ := strconv.ParseInt(parts[1], 10, 64)
					end = e
				}
			}
			if start < 0 || start >= size || end < start {
				w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", size))
				http.Error(w, "invalid range", http.StatusRequestedRangeNotSatisfiable)
				return
			}
			// Seek if underlying is *os.File
			if f, ok := rc.(*os.File); ok {
				_, _ = f.Seek(start, io.SeekStart)
			} else {
				// Fallback: discard bytes
				_, _ = io.CopyN(io.Discard, rc, start)
			}
			length := end - start + 1
			w.Header().Set("Content-Length", fmt.Sprintf("%d", length))
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = io.CopyN(w, rc, length)
			return
		}
		// No Range: stream full content
		w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, rc)
	})

	// OpenAPI: serve openapi.yaml from workspace root
	mux.HandleFunc("/v0/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		fp := path.Join("openapi.yaml")
		b, err := os.ReadFile(fp)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/yaml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
	})

	// Minimal docs page
	mux.HandleFunc("/v0/docs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<!doctype html><html><head><title>fGo API Docs</title></head><body>
<h1>fGo API (v0)</h1>
<ul>
  <li><a href="/v0/openapi.yaml">OpenAPI Spec</a></li>
  <li>Health: GET /v0/health</li>
  <li>Boxes: GET/POST /v0/boxes</li>
  <li>Box: GET /v0/boxes/{box}</li>
  <li>Push Plan: POST /v0/boxes/{box}/push/plan</li>
  <li>Push Finalize: POST /v0/boxes/{box}/push/finalize</li>
  <li>Latest Commit: GET /v0/boxes/{box}/commits/latest?branch=main</li>
  <li>Blobs: HEAD/PUT /v0/blobs/{sha256}</li>
  <li>Files: GET /v0/files/{commit_id}?path=... (Range supported)</li>
</ul>
</body></html>`))
	})
	handler := httpx.Chain(mux, httpx.Recover(), httpx.RequestID(), httpx.Logger(), httpx.CORS(), httpx.Gzip())
	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Fatal(http.ListenAndServe(addr, handler))
}
