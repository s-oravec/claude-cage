package snapshot

import (
	"testing"
	"time"
)

func TestSnapshotStruct(t *testing.T) {
	snap := Snapshot{
		Name:        "test-snapshot",
		CageName:    "mycage",
		Description: "Test snapshot",
		CreatedAt:   time.Now(),
		State:       "shutoff",
	}

	if snap.Name != "test-snapshot" {
		t.Errorf("expected name 'test-snapshot', got '%s'", snap.Name)
	}

	if snap.CageName != "mycage" {
		t.Errorf("expected cage name 'mycage', got '%s'", snap.CageName)
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		msg  string
	}{
		{"ErrCageNotFound", ErrCageNotFound, "cage not found"},
		{"ErrSnapshotNotFound", ErrSnapshotNotFound, "snapshot not found"},
		{"ErrCageRunning", ErrCageRunning, "cage must be stopped for this operation"},
		{"ErrSnapshotExists", ErrSnapshotExists, "snapshot already exists"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.msg {
				t.Errorf("expected '%s', got '%s'", tt.msg, tt.err.Error())
			}
		})
	}
}

func TestExists(t *testing.T) {
	// Test with non-existent cage - should return false
	if Exists("nonexistent-cage", "snap") {
		t.Error("expected false for non-existent cage")
	}
}

func TestCreate_NonexistentCage(t *testing.T) {
	err := Create("nonexistent-cage-12345", "snap", "test")
	if err != ErrCageNotFound {
		t.Errorf("expected ErrCageNotFound, got %v", err)
	}
}

func TestList_NonexistentCage(t *testing.T) {
	_, err := List("nonexistent-cage-12345")
	if err != ErrCageNotFound {
		t.Errorf("expected ErrCageNotFound, got %v", err)
	}
}

func TestRestore_NonexistentCage(t *testing.T) {
	err := Restore("nonexistent-cage-12345", "snap")
	if err != ErrCageNotFound {
		t.Errorf("expected ErrCageNotFound, got %v", err)
	}
}

func TestDelete_NonexistentCage(t *testing.T) {
	err := Delete("nonexistent-cage-12345", "snap")
	if err != ErrCageNotFound {
		t.Errorf("expected ErrCageNotFound, got %v", err)
	}
}
