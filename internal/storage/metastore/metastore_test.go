package metastore

import (
	"testing"
)

func TestBoxModel(t *testing.T) {
	b := Box{
		ID:            "01J123",
		NamespaceID:   "01K456",
		Name:          "gofiles",
		Visibility:    "public",
		DefaultBranch: "main",
	}
	if b.Name != "gofiles" {
		t.Errorf("expected box name gofiles, got %s", b.Name)
	}
}

func TestCommitModel(t *testing.T) {
	c := Commit{
		ID:        "01J789",
		BoxID:     "01J123",
		Branch:    "main",
		ParentID:  nil,
		Message:   "init",
		Author:    "token:abc",
		Timestamp: "2025-10-11T00:00:00Z",
		Entries:   []Entry{{Path: "README.md", SHA256: "abc", Size: 1234, Mode: 420}},
	}
	if c.Branch != "main" {
		t.Errorf("expected branch main, got %s", c.Branch)
	}
	if len(c.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(c.Entries))
	}
}
