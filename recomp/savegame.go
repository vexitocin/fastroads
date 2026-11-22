package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type SaveData struct {
	Vehicle       int     `json:"vehicle"`
	Terrain       Terrain `json:"terrain"`
	World         World   `json:"world"`
	Season        Season  `json:"season"`
	Weather       Weather `json:"weather"`
	Seed          int64   `json:"seed"`
	Ultra         bool    `json:"ultra"`
	AutoDrive     bool    `json:"autoDrive"`
	Headlights    bool    `json:"headlights"`
	AudioEnabled  bool    `json:"audioEnabled"`
	Discord       bool    `json:"discord"`
	CruiseMPH     float64 `json:"cruiseMPH"`
	DriveMode     int     `json:"driveMode"`
	CameraMode    int     `json:"cameraMode"`
	Units         int     `json:"units"`
	BestDistance  float64 `json:"bestDistance"`
	TotalDistance float64 `json:"totalDistance"`
}

func defaultSaveData() SaveData {
	return SaveData{
		Terrain:      TerrainNormal,
		World:        WorldHills,
		Season:       SeasonSummer,
		Seed:         1,
		Headlights:   true,
		AudioEnabled: true,
		Discord:      true,
		CruiseMPH:    90 * metresPerSecondToMPH,
	}
}

func defaultSavePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(configDir, "fastroads", "recomp.json"), nil
}

func loadSave(path string) (SaveData, error) {
	data := defaultSaveData()
	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return data, nil
	}
	if err != nil {
		return data, fmt.Errorf("read save file %q: %w", path, err)
	}
	if err := json.Unmarshal(contents, &data); err != nil {
		return data, fmt.Errorf("parse save file %q: %w", path, err)
	}
	if err := data.validate(); err != nil {
		return data, fmt.Errorf("validate save file %q: %w", path, err)
	}
	return data, nil
}

func saveData(path string, data SaveData) error {
	if err := data.validate(); err != nil {
		return fmt.Errorf("refuse to save invalid profile: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create save directory: %w", err)
	}
	contents, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("encode save file: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".recomp-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary save file: %w", err)
	}
	tempPath := temp.Name()
	if _, err := temp.Write(contents); err != nil {
		_ = temp.Close()
		_ = os.Remove(tempPath)
		return fmt.Errorf("write save file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		_ = os.Remove(tempPath)
		return fmt.Errorf("sync save file: %w", err)
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("close save file: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("replace save file: %w", err)
	}
	return nil
}

func (data SaveData) validate() error {
	if data.Vehicle < 0 || data.Vehicle >= len(Vehicles) {
		return fmt.Errorf("vehicle index %d is out of range", data.Vehicle)
	}
	if data.Terrain >= terrainCount {
		return fmt.Errorf("terrain index %d is out of range", data.Terrain)
	}
	if data.World >= worldCount {
		return fmt.Errorf("world index %d is out of range", data.World)
	}
	if data.Season >= seasonCount {
		return fmt.Errorf("season index %d is out of range", data.Season)
	}
	if int(data.Weather) >= len(weatherOptions(data.World, data.Season)) {
		return fmt.Errorf("weather index %d is out of range", data.Weather)
	}
	if data.DriveMode < 0 || data.DriveMode > 2 {
		return fmt.Errorf("drive mode %d is out of range", data.DriveMode)
	}
	if data.CameraMode < 0 || data.CameraMode > 2 {
		return fmt.Errorf("camera mode %d is out of range", data.CameraMode)
	}
	if data.Units < 0 || data.Units > 1 {
		return fmt.Errorf("units mode %d is out of range", data.Units)
	}
	if data.BestDistance < 0 || data.TotalDistance < 0 {
		return errors.New("distance counters cannot be negative")
	}
	return nil
}
