package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveDataRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile", "recomp.json")
	want := defaultSaveData()
	want.World = WorldVenus
	want.Season = SeasonAutumn
	want.Weather = 1
	want.Seed = 9876
	want.BestDistance = 12.5
	want.TotalDistance = 108.25
	if err := saveData(path, want); err != nil {
		t.Fatal(err)
	}
	want.BestDistance = 13
	if err := saveData(path, want); err != nil {
		t.Fatalf("updating an existing profile failed: %v", err)
	}
	got, err := loadSave(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("loaded profile differs:\n got: %+v\nwant: %+v", got, want)
	}
}

func TestLoadSaveRejectsMalformedAndInvalidData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recomp.json")
	for _, contents := range []string{
		"{",
		`{"vehicle":999}`,
	} {
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadSave(path); err == nil {
			t.Fatalf("expected invalid save to fail: %q", contents)
		}
	}
}
