package main

import (
	"path/filepath"
	"testing"
)

func TestBundledAudioAssetsDecode(t *testing.T) {
	mediaDir, err := findMediaDirectory()
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{
		"ambiance_summer_low.2130513f.mp3",
		"rolling_06.84342ca7.mp3",
		"hit_03.43267a22.mp3",
	} {
		t.Run(file, func(t *testing.T) {
			pcm, err := decodeMP3(filepath.Join(mediaDir, file))
			if err != nil {
				t.Fatal(err)
			}
			if len(pcm) == 0 || len(pcm)%4 != 0 {
				t.Fatalf("decoded audio has invalid stereo PCM length: %d bytes", len(pcm))
			}
		})
	}
}
