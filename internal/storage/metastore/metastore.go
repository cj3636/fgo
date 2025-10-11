package metastore

import (
	"context"
)

type Box struct {
	ID            string
	NamespaceID   string
	Name          string
	Visibility    string
	DefaultBranch string
}

type Commit struct {
	ID        string
	BoxID     string
	Branch    string
	ParentID  *string
	Message   string
	Author    string
	Timestamp string
	Entries   []Entry
}

type Entry struct {
	Path   string
	SHA256 string
	Size   int64
	Mode   int
}

type MetadataStore interface {
	CreateBox(ctx context.Context, b Box) (Box, error)
	GetBox(ctx context.Context, ns, name string) (Box, error)
	SaveCommit(ctx context.Context, c Commit) (Commit, error)
	LatestCommit(ctx context.Context, boxID string, branch string) (Commit, error)
	MoveRef(ctx context.Context, boxID, branch, parentID, newID string) error
	ListPublicBoxes(ctx context.Context) ([]Box, error)
	GetCommitByID(ctx context.Context, id string) (Commit, error)
}
