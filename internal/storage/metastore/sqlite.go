package metastore

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/oklog/ulid"
	_ "modernc.org/sqlite"
)

type SQLiteMetaStore struct {
	db *sql.DB
}

func NewSQLiteMetaStore(path string) (*SQLiteMetaStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// Initialize schema from init.sql if available
	if err := initSchema(db); err != nil {
		return nil, err
	}
	return &SQLiteMetaStore{db: db}, nil
}

func initSchema(db *sql.DB) error {
	// Always ensure minimal schema exists
	if err := fallbackSchema(db); err != nil {
		return err
	}
	// Optionally apply init.sql for extended schema if present, but ignore errors
	if buf, err := os.ReadFile("init.sql"); err == nil {
		stmts := strings.Split(string(buf), ";")
		for _, s := range stmts {
			s = strings.TrimSpace(s)
			if s == "" || strings.HasPrefix(s, "--") {
				continue
			}
			_, _ = db.Exec(s)
		}
	}
	return nil
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// fallbackSchema attempts to create minimal required tables inline
func fallbackSchema(db *sql.DB) error {
	minimal := []string{
		`PRAGMA foreign_keys = ON;`,
		`CREATE TABLE IF NOT EXISTS boxes (
			id TEXT PRIMARY KEY,
			namespace_id TEXT NOT NULL DEFAULT 'global',
			name TEXT NOT NULL,
			visibility TEXT NOT NULL CHECK (visibility IN ('public','unlisted','private')),
			default_arm TEXT NOT NULL DEFAULT 'main',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE(namespace_id, name)
		);`,
		`CREATE TABLE IF NOT EXISTS enacts (
			id TEXT PRIMARY KEY,
			box_id TEXT NOT NULL,
			arm TEXT NOT NULL,
			parent_id TEXT,
			message TEXT,
			author TEXT,
			timestamp TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS entries (
			enact_id TEXT NOT NULL,
			path TEXT NOT NULL,
			sha256 TEXT NOT NULL,
			size INTEGER NOT NULL,
			mode INTEGER NOT NULL,
			PRIMARY KEY (enact_id, path)
		);`,
		`CREATE TABLE IF NOT EXISTS refs (
			box_id TEXT NOT NULL,
			arm TEXT NOT NULL,
			enact_id TEXT NOT NULL,
			PRIMARY KEY (box_id, arm)
		);`,
	}
	for _, stmt := range minimal {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("fallback schema failed: %w (stmt: %s)", err, firstN(stmt, 120))
		}
	}
	return nil
}

// Implement MetadataStore methods
func (s *SQLiteMetaStore) CreateBox(ctx context.Context, b Box) (Box, error) {
	if b.ID == "" {
		b.ID = newULID()
	}
	if b.DefaultBranch == "" {
		b.DefaultBranch = "main"
	}
	if b.Visibility == "" {
		b.Visibility = "public"
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `INSERT INTO boxes(id, namespace_id, name, visibility, default_arm, created_at, updated_at) VALUES(?,?,?,?,?,?,?)`,
		b.ID, b.NamespaceID, b.Name, b.Visibility, b.DefaultBranch, now, now)
	if err != nil {
		return Box{}, err
	}
	return b, nil
}

func (s *SQLiteMetaStore) GetBox(ctx context.Context, ns, name string) (Box, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, namespace_id, name, visibility, default_arm FROM boxes WHERE namespace_id=? AND name=?`, ns, name)
	var b Box
	if err := row.Scan(&b.ID, &b.NamespaceID, &b.Name, &b.Visibility, &b.DefaultBranch); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Box{}, fmt.Errorf("not found")
		}
		return Box{}, err
	}
	return b, nil
}

func (s *SQLiteMetaStore) SaveCommit(ctx context.Context, c Commit) (Commit, error) {
	if c.ID == "" {
		c.ID = newULID()
	}
	if c.Timestamp == "" {
		c.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	// Insert enact
	_, err := s.db.ExecContext(ctx, `INSERT INTO enacts(id, box_id, arm, parent_id, message, author, timestamp) VALUES(?,?,?,?,?,?,?)`,
		c.ID, c.BoxID, c.Branch, c.ParentID, c.Message, c.Author, c.Timestamp)
	if err != nil {
		return Commit{}, err
	}
	// Insert entries
	for _, e := range c.Entries {
		_, err := s.db.ExecContext(ctx, `INSERT INTO entries(enact_id, path, sha256, size, mode) VALUES(?,?,?,?,?)`,
			c.ID, e.Path, e.SHA256, e.Size, e.Mode)
		if err != nil {
			return Commit{}, err
		}
	}
	return c, nil
}

func newULID() string {
	entropy := ulid.Monotonic(rand.Reader, 0)
	id := ulid.MustNew(ulid.Timestamp(time.Now()), entropy)
	return id.String()
}

func (s *SQLiteMetaStore) LatestCommit(ctx context.Context, boxID string, branch string) (Commit, error) {
	row := s.db.QueryRowContext(ctx, `SELECT enact_id FROM refs WHERE box_id=? AND arm=?`, boxID, branch)
	var id string
	if err := row.Scan(&id); err != nil {
		return Commit{}, err
	}
	return s.GetCommitByID(ctx, id)
}

func (s *SQLiteMetaStore) GetCommitByID(ctx context.Context, id string) (Commit, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, box_id, arm, parent_id, message, author, timestamp FROM enacts WHERE id=?`, id)
	var c Commit
	var parent sql.NullString
	if err := row.Scan(&c.ID, &c.BoxID, &c.Branch, &parent, &c.Message, &c.Author, &c.Timestamp); err != nil {
		return Commit{}, err
	}
	if parent.Valid {
		c.ParentID = &parent.String
	}
	// Entries
	rows, err := s.db.QueryContext(ctx, `SELECT path, sha256, size, mode FROM entries WHERE enact_id=?`, id)
	if err != nil {
		return Commit{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.Path, &e.SHA256, &e.Size, &e.Mode); err != nil {
			return Commit{}, err
		}
		c.Entries = append(c.Entries, e)
	}
	return c, nil
}

func (s *SQLiteMetaStore) MoveRef(ctx context.Context, boxID, branch, parentID, newID string) error {
	// Check current ref
	row := s.db.QueryRowContext(ctx, `SELECT enact_id FROM refs WHERE box_id=? AND arm=?`, boxID, branch)
	var cur string
	if err := row.Scan(&cur); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// No ref yet; allow move only if parentID == "" (not provided)
			if parentID != "" {
				return fmt.Errorf("parent mismatch")
			}
			_, err := s.db.ExecContext(ctx, `INSERT INTO refs(box_id, arm, enact_id) VALUES(?,?,?)`, boxID, branch, newID)
			return err
		}
		return err
	}
	if cur != parentID {
		return fmt.Errorf("parent mismatch")
	}
	_, err := s.db.ExecContext(ctx, `UPDATE refs SET enact_id=? WHERE box_id=? AND arm=?`, newID, boxID, branch)
	return err
}

func (s *SQLiteMetaStore) ListPublicBoxes(ctx context.Context) ([]Box, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, namespace_id, name, visibility, default_arm FROM boxes WHERE visibility='public'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Box
	for rows.Next() {
		var b Box
		if err := rows.Scan(&b.ID, &b.NamespaceID, &b.Name, &b.Visibility, &b.DefaultBranch); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, nil
}
