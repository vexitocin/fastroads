package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/mp3"
)

const audioSampleRate = 44100

type AudioSystem struct {
	context *audio.Context
	tracks  map[string]*audio.Player
	crash   *audio.Player
}

func NewAudioSystem(mediaDir string) (*AudioSystem, error) {
	system := &AudioSystem{
		context: audio.NewContext(audioSampleRate),
		tracks:  make(map[string]*audio.Player),
	}
	loopFiles := map[string]string{
		"summer":       "ambiance_summer_low.2130513f.mp3",
		"rain":         "ambiance_cloudy.4d5b0a33.mp3",
		"offworld":     "ambiance_offworld.41a010f6.mp3",
		"rolling":      "rolling_06.84342ca7.mp3",
		"offroad":      "rolling_offroad_04.e2ad11f4.mp3",
		"acceleration": "veh_accel_06.8478c426.mp3",
	}
	for name, file := range loopFiles {
		player, err := loadLoopPlayer(system.context, filepath.Join(mediaDir, file))
		if err != nil {
			system.Close()
			return nil, err
		}
		system.tracks[name] = player
		player.SetVolume(0)
	}
	crash, err := loadSoundPlayer(system.context, filepath.Join(mediaDir, "hit_03.43267a22.mp3"))
	if err != nil {
		system.Close()
		return nil, err
	}
	system.crash = crash
	system.tracks["summer"].SetVolume(0.16)
	system.SetEnabled(true)
	return system, nil
}

func loadLoopPlayer(context *audio.Context, path string) (*audio.Player, error) {
	pcm, err := decodeMP3(path)
	if err != nil {
		return nil, err
	}
	loop := audio.NewInfiniteLoop(bytes.NewReader(pcm), int64(len(pcm)))
	player, err := context.NewPlayer(loop)
	if err != nil {
		return nil, fmt.Errorf("create audio player for %q: %w", path, err)
	}
	return player, nil
}

func loadSoundPlayer(context *audio.Context, path string) (*audio.Player, error) {
	pcm, err := decodeMP3(path)
	if err != nil {
		return nil, err
	}
	player, err := context.NewPlayer(bytes.NewReader(pcm))
	if err != nil {
		return nil, fmt.Errorf("create audio player for %q: %w", path, err)
	}
	return player, nil
}

func decodeMP3(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open audio asset %q: %w", path, err)
	}
	stream, decodeErr := mp3.DecodeWithSampleRate(audioSampleRate, file)
	if decodeErr != nil {
		_ = file.Close()
		return nil, fmt.Errorf("decode audio asset %q: %w", path, decodeErr)
	}
	pcm, readErr := io.ReadAll(stream)
	closeErr := file.Close()
	if readErr != nil {
		return nil, fmt.Errorf("read decoded audio asset %q: %w", path, readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close audio asset %q: %w", path, closeErr)
	}
	return pcm, nil
}

func (a *AudioSystem) SetEnabled(enabled bool) {
	for _, player := range a.tracks {
		if enabled {
			if !player.IsPlaying() {
				player.Play()
			}
		} else {
			player.Pause()
		}
	}
}

func (a *AudioSystem) Update(state DriveState, environment Environment, enabled bool) {
	if !enabled {
		a.SetEnabled(false)
		return
	}
	a.SetEnabled(true)
	for _, name := range []string{"summer", "rain", "offworld"} {
		a.tracks[name].SetVolume(0)
	}
	ambience := "summer"
	switch environment.AmbientAsset {
	case "ambiance_cloudy.4d5b0a33.mp3":
		ambience = "rain"
	case "ambiance_offworld.41a010f6.mp3":
		ambience = "offworld"
	}
	a.tracks[ambience].SetVolume(0.16)

	speed := absFloat(state.SpeedMPH)
	rollingVolume := 0.02 + minFloat(speed/180, 0.2)
	roadTrack, offroadTrack := a.tracks["rolling"], a.tracks["offroad"]
	if state.OffRoad {
		roadTrack.SetVolume(0)
		offroadTrack.SetVolume(rollingVolume)
	} else {
		roadTrack.SetVolume(rollingVolume)
		offroadTrack.SetVolume(0)
	}
	a.tracks["acceleration"].SetVolume(minFloat(absFloat(state.Drive)/300, 0.16))
}

func (a *AudioSystem) PlayCrash() error {
	if err := a.crash.Rewind(); err != nil {
		return fmt.Errorf("rewind collision sound: %w", err)
	}
	a.crash.Play()
	return nil
}

func (a *AudioSystem) Close() {
	for _, player := range a.tracks {
		player.Close()
	}
	if a.crash != nil {
		a.crash.Close()
	}
}

func absFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
