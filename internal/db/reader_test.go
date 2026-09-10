package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverProfiles(t *testing.T) {
	tmpHome, err := os.MkdirTemp("", "omp-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpHome)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	reader := NewReader()

	// 1. When ~/.omp does not exist, return 0 profiles
	profiles := reader.DiscoverProfiles()
	if len(profiles) != 0 {
		t.Fatalf("expected 0 profiles when .omp missing, got %d", len(profiles))
	}

	// 2. When ~/.omp directory exists (even before stats.db is created), default profile must be registered
	ompDir := filepath.Join(tmpHome, ".omp")
	if err := os.MkdirAll(ompDir, 0755); err != nil {
		t.Fatal(err)
	}

	profiles = reader.DiscoverProfiles()
	expectedPath := filepath.Join(ompDir, "stats.db")
	if p, ok := profiles["default"]; !ok || p != expectedPath {
		t.Fatalf("expected default profile at %s, got %v", expectedPath, profiles)
	}
}
