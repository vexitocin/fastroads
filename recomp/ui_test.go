package main

import "testing"

func TestSettingsNavigationReturnsToPreviousScreen(t *testing.T) {
	game := &Game{screen: UIScreenPaused}
	game.applyUIAction(actionSettings)
	if game.screen != UIScreenSettings || game.returnTo != UIScreenPaused {
		t.Fatalf("settings navigation lost its origin: screen=%d returnTo=%d", game.screen, game.returnTo)
	}
	game.applyUIAction(actionBack)
	if game.screen != UIScreenPaused {
		t.Fatalf("back returned to screen %d, want paused screen", game.screen)
	}
}

func TestSettingsActionsUpdateSelections(t *testing.T) {
	game := &Game{vehicle: len(Vehicles) - 1, terrain: TerrainNormal}
	game.applyUIAction(actionCycleVehicle)
	if game.vehicle != 0 {
		t.Fatalf("vehicle selection did not wrap: got %d", game.vehicle)
	}
	game.applyUIAction(actionCycleTerrain)
	if game.terrain != TerrainHard {
		t.Fatalf("terrain selection did not advance: got %s", game.terrain)
	}
	game.applyUIAction(actionToggleUnits)
	if game.units != 1 {
		t.Fatalf("expected metric units, got %d", game.units)
	}
}

func TestWorldAndWeatherSettingsCycleValidOptions(t *testing.T) {
	game := &Game{world: WorldHills, season: SeasonWinter}
	game.applyUIAction(actionCycleWeather)
	game.applyUIAction(actionCycleWeather)
	if !environmentFor(game.world, game.season, game.weather).Snow {
		t.Fatal("winter weather cycle should select snow after sunrise")
	}
	game.applyUIAction(actionCycleWorld)
	if game.world != WorldMars || game.weather != 0 {
		t.Fatalf("world selection did not reset weather: world=%s weather=%d", game.world, game.weather)
	}
	game.applyUIAction(actionCycleWeather)
	if int(game.weather) >= len(weatherOptions(game.world, game.season)) {
		t.Fatalf("selected weather index %d is invalid for %s", game.weather, game.world)
	}
}
