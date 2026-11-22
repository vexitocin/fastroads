package main

import (
	"fmt"
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type UIScreen uint8

const (
	UIScreenMenu UIScreen = iota
	UIScreenPlaying
	UIScreenPaused
	UIScreenSettings
	UIScreenControls
)

type uiAction uint8

const (
	actionStart uiAction = iota
	actionSettings
	actionControls
	actionResume
	actionMainMenu
	actionCycleVehicle
	actionCycleTerrain
	actionCycleWorld
	actionCycleSeason
	actionCycleWeather
	actionCycleSeed
	actionToggleUltra
	actionCycleCamera
	actionCycleDrive
	actionToggleLights
	actionToggleUnits
	actionToggleAudio
	actionToggleDiscord
	actionBack
)

type uiButton struct {
	label  string
	rect   image.Rectangle
	action uiAction
}

func (g *Game) setScreen(screen UIScreen) {
	g.screen = screen
	g.uiFocus = 0
}

func (g *Game) updateUI() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		switch g.screen {
		case UIScreenPaused:
			g.setScreen(UIScreenPlaying)
		case UIScreenSettings, UIScreenControls:
			g.setScreen(g.returnTo)
		}
		return nil
	}
	if g.screen == UIScreenPaused && inpututil.IsKeyJustPressed(ebiten.KeyP) {
		g.setScreen(UIScreenPlaying)
		return nil
	}
	buttons := g.uiButtons(screenWidth, screenHeight)
	if len(buttons) == 0 {
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) || inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		g.uiFocus = (g.uiFocus + 1) % len(buttons)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyUp) || inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
		g.uiFocus = (g.uiFocus + len(buttons) - 1) % len(buttons)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		g.applyUIAction(buttons[g.uiFocus].action)
		return nil
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := uiCursorPosition(screenWidth, screenHeight)
		for i, button := range buttons {
			if image.Pt(x, y).In(button.rect) {
				g.uiFocus = i
				g.applyUIAction(button.action)
				break
			}
		}
	}
	return nil
}

func (g *Game) applyUIAction(action uiAction) {
	switch action {
	case actionStart, actionResume:
		g.setScreen(UIScreenPlaying)
	case actionSettings:
		g.returnTo = g.screen
		g.setScreen(UIScreenSettings)
	case actionControls:
		g.returnTo = g.screen
		g.setScreen(UIScreenControls)
	case actionMainMenu:
		g.autoDrive = false
		g.setScreen(UIScreenMenu)
		g.saveDirty = true
	case actionCycleVehicle:
		g.vehicle = (g.vehicle + 1) % len(Vehicles)
		g.state = DriveState{}
		g.saveDirty = true
	case actionCycleTerrain:
		g.terrain = (g.terrain + 1) % terrainCount
		g.saveDirty = true
	case actionCycleWorld:
		g.world = (g.world + 1) % worldCount
		g.weather = 0
		g.saveDirty = true
	case actionCycleSeason:
		g.season = (g.season + 1) % seasonCount
		if int(g.weather) >= len(weatherOptions(g.world, g.season)) {
			g.weather = 0
		}
		g.saveDirty = true
	case actionCycleWeather:
		options := weatherOptions(g.world, g.season)
		g.weather = Weather((int(g.weather) + 1) % len(options))
		g.saveDirty = true
	case actionCycleSeed:
		g.seed++
		g.saveDirty = true
	case actionToggleUltra:
		g.ultra = !g.ultra
		g.saveDirty = true
	case actionCycleCamera:
		g.cameraMode = (g.cameraMode + 1) % 3
		g.saveDirty = true
	case actionCycleDrive:
		g.driveMode = (g.driveMode + 1) % 3
		g.saveDirty = true
	case actionToggleLights:
		g.headlights = !g.headlights
		g.saveDirty = true
	case actionToggleUnits:
		g.units = (g.units + 1) % 2
		g.saveDirty = true
	case actionToggleAudio:
		g.audioEnabled = !g.audioEnabled
		if g.audio != nil {
			g.audio.SetEnabled(g.audioEnabled)
		}
		g.saveDirty = true
	case actionToggleDiscord:
		g.discord = !g.discord
		if g.discord {
			g.presence = NewDiscordPresence()
		} else if g.presence != nil {
			g.presence.Close()
			g.presence = nil
		}
		g.saveDirty = true
	case actionBack:
		g.setScreen(g.returnTo)
	}
}

func (g *Game) uiButtons(width, height int) []uiButton {
	centerX, centerY := width/2, height/2
	buttonWidth := 360
	buttons := []uiButton{}
	switch g.screen {
	case UIScreenMenu:
		buttons = append(buttons,
			uiButton{"Start Driving", image.Rect(centerX-buttonWidth/2, centerY-35, centerX+buttonWidth/2, centerY+13), actionStart},
			uiButton{"Settings", image.Rect(centerX-buttonWidth/2, centerY+27, centerX+buttonWidth/2, centerY+75), actionSettings},
			uiButton{"Controls", image.Rect(centerX-buttonWidth/2, centerY+89, centerX+buttonWidth/2, centerY+137), actionControls},
		)
	case UIScreenPaused:
		buttons = append(buttons,
			uiButton{"Resume", image.Rect(centerX-buttonWidth/2, centerY-55, centerX+buttonWidth/2, centerY-7), actionResume},
			uiButton{"Settings", image.Rect(centerX-buttonWidth/2, centerY+7, centerX+buttonWidth/2, centerY+55), actionSettings},
			uiButton{"Main Menu", image.Rect(centerX-buttonWidth/2, centerY+69, centerX+buttonWidth/2, centerY+117), actionMainMenu},
		)
	case UIScreenSettings:
		buttonWidth, buttonHeight, gap := 460, 36, 5
		options := weatherOptions(g.world, g.season)
		firstY := centerY - (14*buttonHeight+13*gap)/2
		add := func(label string, action uiAction) {
			y := firstY + len(buttons)*(buttonHeight+gap)
			buttons = append(buttons, uiButton{label, image.Rect(centerX-buttonWidth/2, y, centerX+buttonWidth/2, y+buttonHeight), action})
		}
		add(fmt.Sprintf("Vehicle: %s", Vehicles[g.vehicle].Name), actionCycleVehicle)
		add(fmt.Sprintf("Terrain: %s", g.terrain), actionCycleTerrain)
		add(fmt.Sprintf("World: %s", g.world), actionCycleWorld)
		add(fmt.Sprintf("Season: %s", g.season), actionCycleSeason)
		add(fmt.Sprintf("Weather: %s", options[int(g.weather)%len(options)]), actionCycleWeather)
		add(fmt.Sprintf("World seed: %d", g.seed), actionCycleSeed)
		add(fmt.Sprintf("Ultra+: %s", enabledLabel(g.ultra)), actionToggleUltra)
		add(fmt.Sprintf("Camera: %s", cameraName(g.cameraMode)), actionCycleCamera)
		add(fmt.Sprintf("Drive mode: %s", driveModeName(g.driveMode)), actionCycleDrive)
		add(fmt.Sprintf("Headlights: %s", enabledLabel(g.headlights)), actionToggleLights)
		add(fmt.Sprintf("Audio: %s", enabledLabel(g.audioEnabled)), actionToggleAudio)
		add(fmt.Sprintf("Discord Rich Presence: %s", enabledLabel(g.discord)), actionToggleDiscord)
		add(fmt.Sprintf("Speed units: %s", unitsName(g.units)), actionToggleUnits)
		add("Back", actionBack)
	case UIScreenControls:
		buttons = append(buttons,
			uiButton{"Back", image.Rect(centerX-buttonWidth/2, centerY+154, centerX+buttonWidth/2, centerY+202), actionBack},
		)
	}
	return buttons
}

func (g *Game) drawUI(screen *ebiten.Image) {
	w, h := screen.Bounds().Dx(), screen.Bounds().Dy()
	buttons := g.uiButtons(w, h)
	drawRect(screen, 0, 0, float64(w), float64(h), color.RGBA{5, 9, 15, 195})
	panel := uiPanelBounds(buttons, w, h, g.screen)
	drawRect(screen, float64(panel.Min.X), float64(panel.Min.Y), float64(panel.Dx()), float64(panel.Dy()), color.RGBA{24, 31, 40, 245})

	title := "FAST ROADS"
	subtitle := "Native Go edition"
	switch g.screen {
	case UIScreenPaused:
		title, subtitle = "PAUSED", "Take a breather."
	case UIScreenSettings:
		title, subtitle = "SETTINGS", "Choose your drive."
	case UIScreenControls:
		title, subtitle = "CONTROLS", "Keyboard and mouse"
	}
	ebitenutil.DebugPrintAt(screen, title, panel.Min.X+28, panel.Min.Y+22)
	ebitenutil.DebugPrintAt(screen, subtitle, panel.Min.X+28, panel.Min.Y+42)

	mouseX, mouseY := uiCursorPosition(w, h)
	for i, button := range buttons {
		hovered := image.Pt(mouseX, mouseY).In(button.rect)
		background := color.RGBA{51, 61, 72, 255}
		if i == g.uiFocus || hovered {
			background = color.RGBA{197, 78, 42, 255}
		}
		drawRect(screen, float64(button.rect.Min.X), float64(button.rect.Min.Y), float64(button.rect.Dx()), float64(button.rect.Dy()), background)
		ebitenutil.DebugPrintAt(screen, button.label, button.rect.Min.X+16, button.rect.Min.Y+(button.rect.Dy()-16)/2)
	}

	if g.screen == UIScreenControls {
		controls := []string{
			"W / Up: accelerate",
			"S / Down: brake or reverse",
			"A / Left, D / Right: steer",
			"Space: handbrake    Shift: boost",
			"F: autodrive    =: cruise control",
			"B: drivetrain    C: camera    H / L: headlights",
			"1-5: vehicle    T: terrain    U: Ultra+    Y: world",
			"V: season    G: weather    N: next seed",
			"P / Esc: pause    R: reset",
			"Click UI buttons or use Up/Down and Enter.",
		}
		for i, line := range controls {
			ebitenutil.DebugPrintAt(screen, line, panel.Min.X+28, panel.Min.Y+82+i*18)
		}
	}
}

func uiPanelBounds(buttons []uiButton, width, height int, screen UIScreen) image.Rectangle {
	if screen == UIScreenControls {
		return image.Rect(width/2-260, height/2-230, width/2+260, height/2+230)
	}
	if len(buttons) == 0 {
		return image.Rect(width/2-200, height/2-100, width/2+200, height/2+100)
	}
	top, bottom := buttons[0].rect.Min.Y, buttons[0].rect.Max.Y
	for _, button := range buttons[1:] {
		top = min(top, button.rect.Min.Y)
		bottom = max(bottom, button.rect.Max.Y)
	}
	return image.Rect(width/2-260, top-78, width/2+260, bottom+32)
}

func uiCursorPosition(width, height int) (int, int) {
	x, y := ebiten.CursorPosition()
	windowWidth, windowHeight := ebiten.WindowSize()
	if windowWidth <= 0 || windowHeight <= 0 {
		return x, y
	}
	return x * width / windowWidth, y * height / windowHeight
}

func enabledLabel(enabled bool) string {
	if enabled {
		return "On"
	}
	return "Off"
}

func cameraName(mode int) string {
	return []string{"Chase", "Chase Far", "First Person"}[mode]
}

func driveModeName(mode int) string {
	return []string{"AWD", "FWD", "RWD"}[mode]
}

func unitsName(units int) string {
	return []string{"MPH / miles", "KM/H / km"}[units]
}
