package main

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
)

type VehicleAssets struct {
	Body    *Mesh
	Texture *ebiten.Image
}

func loadVehicleAssets() ([]VehicleAssets, error) {
	mediaDir, err := findMediaDirectory()
	if err != nil {
		return nil, err
	}

	meshes := make(map[string]*Mesh)
	textures := make(map[string]*ebiten.Image)
	assets := make([]VehicleAssets, len(Vehicles))
	for i, vehicle := range Vehicles {
		body := meshes[vehicle.BodyAsset]
		if body == nil {
			body, err = LoadOBJ(filepath.Join(mediaDir, vehicle.BodyAsset))
			if err != nil {
				return nil, err
			}
			meshes[vehicle.BodyAsset] = body
		}
		assets[i].Body = body
		if vehicle.TextureAsset == "" {
			continue
		}
		texture := textures[vehicle.TextureAsset]
		if texture == nil {
			path := filepath.Join(mediaDir, vehicle.TextureAsset)
			file, err := os.Open(path)
			if err != nil {
				return nil, fmt.Errorf("open vehicle texture %q: %w", path, err)
			}
			decoded, _, decodeErr := image.Decode(file)
			closeErr := file.Close()
			if decodeErr != nil {
				return nil, fmt.Errorf("decode vehicle texture %q: %w", path, decodeErr)
			}
			if closeErr != nil {
				return nil, fmt.Errorf("close vehicle texture %q: %w", path, closeErr)
			}
			texture = ebiten.NewImageFromImage(decoded)
			textures[vehicle.TextureAsset] = texture
		}
		assets[i].Texture = texture
	}
	return assets, nil
}

func findMediaDirectory() (string, error) {
	bases := make([]string, 0, 2)
	if workingDir, err := os.Getwd(); err == nil {
		bases = append(bases, workingDir)
	}
	if executable, err := os.Executable(); err == nil {
		bases = append(bases, filepath.Dir(executable))
	}
	for _, base := range bases {
		for _, candidate := range []string{
			filepath.Join(base, "static", "media"),
			filepath.Join(base, "..", "static", "media"),
			filepath.Join(base, "..", "..", "static", "media"),
		} {
			if info, err := os.Stat(filepath.Join(candidate, Vehicles[0].BodyAsset)); err == nil && !info.IsDir() {
				return candidate, nil
			}
		}
	}
	return "", fmt.Errorf("could not locate the bundled static/media vehicle assets")
}
