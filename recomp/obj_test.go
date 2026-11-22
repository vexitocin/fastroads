package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParseOBJTriangulatesPolygonAndResolvesNegativeIndices(t *testing.T) {
	input := `
v 0 0 0
v 2 0 0
v 2 2 0
v 0 2 0
vt 0 0
vt 1 0
vt 1 1
vt 0 1
f -4/1 -3/2 -2/3 -1/4
`
	mesh, err := ParseOBJ(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(mesh.Triangles) != 2 {
		t.Fatalf("got %d triangles, want 2", len(mesh.Triangles))
	}
	if mesh.Max.X != 2 || mesh.Max.Y != 2 {
		t.Fatalf("unexpected bounds: min=%+v max=%+v", mesh.Min, mesh.Max)
	}
	if mesh.Triangles[0].Vertices[1].UV.X != 1 {
		t.Fatalf("texture coordinates were not preserved: %+v", mesh.Triangles[0].Vertices[1].UV)
	}
}

func TestParseOBJRejectsOutOfRangeFaceIndices(t *testing.T) {
	_, err := ParseOBJ(strings.NewReader("v 0 0 0\nv 1 0 0\nv 0 1 0\nf 1 2 4\n"))
	if err == nil {
		t.Fatal("expected invalid face index to fail")
	}
}

func TestBundledVehicleMeshesAreReadable(t *testing.T) {
	mediaDir, err := findMediaDirectory()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, vehicle := range Vehicles {
		if seen[vehicle.BodyAsset] {
			continue
		}
		seen[vehicle.BodyAsset] = true
		mesh, err := LoadOBJ(filepath.Join(mediaDir, vehicle.BodyAsset))
		if err != nil {
			t.Fatal(err)
		}
		if len(mesh.Triangles) == 0 || mesh.Max.X <= mesh.Min.X {
			t.Fatalf("invalid mesh bounds or faces in %s", vehicle.BodyAsset)
		}
	}
}
