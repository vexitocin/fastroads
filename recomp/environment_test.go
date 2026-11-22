package main

import "testing"

func TestWeatherOptionsAndEnvironmentEffects(t *testing.T) {
	winter := weatherOptions(WorldHills, SeasonWinter)
	if winter[2] != "Snow" {
		t.Fatalf("winter weather options should include snow, got %v", winter)
	}
	env := environmentFor(WorldHills, SeasonWinter, Weather(2))
	if !env.Snow || env.OffRoadGrip >= 0.5 {
		t.Fatalf("snow should reduce off-road grip: %+v", env)
	}
	if !environmentFor(WorldMoon, SeasonSummer, 0).Headlights {
		t.Fatal("moon environment should require headlights")
	}
}

func TestSeededNoiseIsDeterministicAndSeedSensitive(t *testing.T) {
	first := seededNoise(42, 3.25)
	if first != seededNoise(42, 3.25) {
		t.Fatal("same seed and coordinate should produce the same value")
	}
	if first == seededNoise(43, 3.25) {
		t.Fatal("different seeds should produce different terrain values")
	}
}
