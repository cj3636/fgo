package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strings"

	"fgo/internal/storage/blobstore"
	"fgo/internal/storage/metastore"
)

func main() {
	fmt.Println("gofile server starting...")

	// Initialize BlobStoreFS and SQLiteMetaStore
	blobRoot := "./blobs"
	_ = os.MkdirAll(blobRoot, 0755)
	blobs := blobstore.NewBlobStoreFS(blobRoot)
	meta, err := metastore.NewSQLiteMetaStore("./meta.db")
	if err != nil {
		log.Fatalf("failed to open metastore: %v", err)
	}

	// Health
	http.HandleFunc("/v0/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Boxes list/create only at exact /v0/boxes (no trailing slash)
	http.HandleFunc("/v0/boxes", func(w http.ResponseWriter, r *http.Request) {
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
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "bad json", http.StatusBadRequest)
				return
			}
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
	http.HandleFunc("/v0/boxes/", func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/v0/boxes/")
		parts := strings.Split(p, "/")
		if len(parts) < 2 {
			http.NotFound(w, r)
			return
		}
		boxName := parts[0]
		action := strings.Join(parts[1:], "/")
		box, err := meta.GetBox(r.Context(), "global", boxName)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		switch {
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
				ParentCommitID *string           `json:"parent_commit_id"`
				Message        string            `json:"message"`
				Entries        []metastore.Entry `json:"entries"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "bad json", http.StatusBadRequest)
				return
			}
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
			commit := metastore.Commit{BoxID: box.ID, Branch: req.Branch, ParentID: req.ParentCommitID, Message: req.Message, Author: "", Entries: req.Entries}
			commit, err = meta.SaveCommit(r.Context(), commit)
			if err != nil {
				http.Error(w, "error", http.StatusInternalServerError)
				return
			}
			parent := ""
			if req.ParentCommitID != nil {
				parent = *req.ParentCommitID
			}
			if err := meta.MoveRef(r.Context(), box.ID, req.Branch, parent, commit.ID); err != nil {
				http.Error(w, "conflict", http.StatusConflict)
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
	http.HandleFunc("/v0/blobs/", func(w http.ResponseWriter, r *http.Request) {
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

	// Files: GET /v0/files/{commit_id}?path=...
	http.HandleFunc("/v0/files/", func(w http.ResponseWriter, r *http.Request) {
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
		w.Header().Set("ETag", "W/\"sha256:"+entry.SHA256+"\"")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, rc)
	})

	// OpenAPI: serve openapi.yaml from workspace root
	http.HandleFunc("/v0/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
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

	log.Fatal(http.ListenAndServe(":8080", nil))
}
