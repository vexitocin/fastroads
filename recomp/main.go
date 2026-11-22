package main

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

const (
	screenWidth  = 1280
	screenHeight = 800
)

type Game struct {
	state        DriveState
	assets       []VehicleAssets
	audio        *AudioSystem
	presence     *DiscordPresence
	screen       UIScreen
	returnTo     UIScreen
	uiFocus      int
	vehicle      int
	terrain      Terrain
	world        World
	season       Season
	weather      Weather
	seed         int64
	ultra        bool
	autoDrive    bool
	headlights   bool
	audioEnabled bool
	discord      bool
	cruiseMPH    float64
	driveMode    int
	cameraMode   int
	units        int
	savePath     string
	saveElapsed  time.Duration
	saveDirty    bool
	profile      SaveData
}

func NewGame() (*Game, error) {
	mediaDir, err := findMediaDirectory()
	if err != nil {
		return nil, err
	}
	assets, err := loadVehicleAssets()
	if err != nil {
		return nil, err
	}
	savePath, err := defaultSavePath()
	if err != nil {
		return nil, err
	}
	profile, err := loadSave(savePath)
	if err != nil {
		return nil, err
	}
	audioSystem, err := NewAudioSystem(mediaDir)
	if err != nil {
		log.Printf("audio unavailable; continuing without sound: %v", err)
	}
	game := &Game{
		assets:       assets,
		audio:        audioSystem,
		screen:       UIScreenMenu,
		terrain:      profile.Terrain,
		world:        profile.World,
		season:       profile.Season,
		weather:      profile.Weather,
		seed:         profile.Seed,
		vehicle:      profile.Vehicle,
		ultra:        profile.Ultra,
		autoDrive:    profile.AutoDrive,
		headlights:   profile.Headlights,
		audioEnabled: profile.AudioEnabled,
		discord:      profile.Discord,
		cruiseMPH:    profile.CruiseMPH,
		driveMode:    profile.DriveMode,
		cameraMode:   profile.CameraMode,
		units:        profile.Units,
		savePath:     savePath,
		profile:      profile,
	}
	if game.audio != nil {
		game.audio.SetEnabled(game.audioEnabled)
	}
	if game.discord {
		game.presence = NewDiscordPresence()
	}
	return game, nil
}

func (g *Game) Update() error {
	if g.screen != UIScreenPlaying {
		if err := g.updateUI(); err != nil {
			return err
		}
		g.updatePresence()
		if g.audio != nil {
			g.audio.Update(DriveState{}, environmentFor(g.world, g.season, g.weather), g.audioEnabled)
		}
		return g.persist(false)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || inpututil.IsKeyJustPressed(ebiten.KeyP) {
		g.setScreen(UIScreenPaused)
		g.saveDirty = true
		return g.persist(false)
	}
	if inpututil.IsKeyJustPressed(ebiten.Key1) {
		g.vehicle = 0
		g.saveDirty = true
	}
	if inpututil.IsKeyJustPressed(ebiten.Key2) {
		g.vehicle = 1
		g.saveDirty = true
	}
	if inpututil.IsKeyJustPressed(ebiten.Key3) {
		g.vehicle = 2
		g.saveDirty = true
	}
	if inpututil.IsKeyJustPressed(ebiten.Key4) {
		g.vehicle = 3
		g.saveDirty = true
	}
	if inpututil.IsKeyJustPressed(ebiten.Key5) {
		g.vehicle = 4
		g.saveDirty = true
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyT) {
		g.terrain = (g.terrain + 1) % terrainCount
		g.saveDirty = true
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyU) {
		g.ultra = !g.ultra
		g.saveDirty = true
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyY) {
		g.world = (g.world + 1) % worldCount
		g.weather = 0
		g.saveDirty = true
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyV) {
		g.season = (g.season + 1) % seasonCount
		g.weather = 0
		g.saveDirty = true
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyG) {
		options := weatherOptions(g.world, g.season)
		g.weather = Weather((int(g.weather) + 1) % len(options))
		g.saveDirty = true
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyN) {
		g.seed++
		g.saveDirty = true
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF) {
		g.autoDrive = !g.autoDrive
		g.saveDirty = true
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEqual) {
		if g.cruiseMPH == 0 {
			g.cruiseMPH = math.Min(Vehicles[g.vehicle].TopSpeedMPH, math.Abs(g.state.SpeedMPH))
			if g.cruiseMPH == 0 {
				g.cruiseMPH = math.Min(90*metresPerSecondToMPH, Vehicles[g.vehicle].TopSpeedMPH)
			}
		} else {
			g.cruiseMPH = 0
		}
		g.saveDirty = true
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyB) {
		g.driveMode = (g.driveMode + 1) % 3
		g.saveDirty = true
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyH) || inpututil.IsKeyJustPressed(ebiten.KeyL) {
		g.headlights = !g.headlights
		g.saveDirty = true
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyC) {
		g.cameraMode = (g.cameraMode + 1) % 3
		g.saveDirty = true
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		g.state = DriveState{}
		g.autoDrive = false
		g.saveDirty = true
	}
	dt := 1.0 / float64(ebiten.TPS())
	env := environmentFor(g.world, g.season, g.weather)
	input := DriveInput{
		Accelerate:  ebiten.IsKeyPressed(ebiten.KeyW) || ebiten.IsKeyPressed(ebiten.KeyArrowUp),
		Reverse:     ebiten.IsKeyPressed(ebiten.KeyS) || ebiten.IsKeyPressed(ebiten.KeyArrowDown),
		Handbrake:   ebiten.IsKeyPressed(ebiten.KeySpace),
		SteerLeft:   ebiten.IsKeyPressed(ebiten.KeyA) || ebiten.IsKeyPressed(ebiten.KeyArrowLeft),
		SteerRight:  ebiten.IsKeyPressed(ebiten.KeyD) || ebiten.IsKeyPressed(ebiten.KeyArrowRight),
		Boost:       ebiten.IsKeyPressed(ebiten.KeyShiftLeft),
		AutoDrive:   g.autoDrive,
		CruiseMPH:   g.cruiseMPH,
		DriveMode:   g.driveMode,
		RoadWidth:   roadWidthMeters(g.terrain),
		OffRoadGrip: env.OffRoadGrip,
	}
	oldCrashes := g.state.Crashes
	g.state = StepDrive(g.state, Vehicles[g.vehicle], input, dt)
	if g.state.Distance > g.profile.BestDistance {
		g.profile.BestDistance = g.state.Distance
	}
	g.profile.TotalDistance += math.Abs(g.state.SpeedMPH) * dt / 3600
	if g.state.Crashes != oldCrashes && g.audio != nil && g.audioEnabled {
		if err := g.audio.PlayCrash(); err != nil {
			return err
		}
	}
	if g.audio != nil {
		g.audio.Update(g.state, env, g.audioEnabled)
	}
	g.saveElapsed += time.Second / time.Duration(ebiten.TPS())
	g.updatePresence()
	if g.saveElapsed >= 15*time.Second {
		g.saveDirty = true
	}
	if err := g.persist(false); err != nil {
		return err
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	g.drawWorld(screen, g.screen == UIScreenPlaying || g.screen == UIScreenPaused)
	if g.screen != UIScreenPlaying {
		g.drawUI(screen)
	}
}

func (g *Game) drawWorld(screen *ebiten.Image, showHUD bool) {
	w, h := screen.Bounds().Dx(), screen.Bounds().Dy()
	horizon := int(float64(h) * 0.39)
	env := environmentFor(g.world, g.season, g.weather)
	drawScene(screen, w, h, horizon, g.state, g.terrain, g.world, env, g.seed, g.ultra, g.headlights || env.Headlights)
	drawVehicle(screen, w, h, Vehicles[g.vehicle], g.assets[g.vehicle], g.state.Lateral, g.state.Steer, g.vehicle, g.cameraMode)
	if showHUD {
		drawHUD(screen, g, w, h)
	}
}

func (g *Game) Layout(_, _ int) (int, int) {
	return screenWidth, screenHeight
}

func main() {
	game, err := NewGame()
	if err != nil {
		log.Fatal(err)
	}
	defer game.Close()
	ebiten.SetWindowTitle("fastroads")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetTPS(60)
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}

func (g *Game) Close() {
	if err := g.persist(true); err != nil {
		log.Printf("save game profile: %v", err)
	}
	if g.presence != nil {
		g.presence.Close()
	}
	if g.audio != nil {
		g.audio.Close()
	}
}

func (g *Game) saveData() SaveData {
	data := g.profile
	data.Vehicle, data.Terrain = g.vehicle, g.terrain
	data.World, data.Season, data.Weather = g.world, g.season, g.weather
	data.Seed, data.Ultra, data.AutoDrive = g.seed, g.ultra, g.autoDrive
	data.Headlights, data.AudioEnabled, data.Discord = g.headlights, g.audioEnabled, g.discord
	data.CruiseMPH, data.DriveMode = g.cruiseMPH, g.driveMode
	data.CameraMode, data.Units = g.cameraMode, g.units
	return data
}

func (g *Game) persist(force bool) error {
	if !force && !g.saveDirty {
		return nil
	}
	if err := saveData(g.savePath, g.saveData()); err != nil {
		return err
	}
	g.profile = g.saveData()
	g.saveDirty = false
	g.saveElapsed = 0
	return nil
}

func (g *Game) updatePresence() {
	if g.presence == nil {
		return
	}
	details := "Browsing the menu"
	switch g.screen {
	case UIScreenPlaying:
		details = "Driving a " + Vehicles[g.vehicle].Name
	case UIScreenPaused:
		details = "Paused"
	case UIScreenSettings:
		details = "Adjusting settings"
	case UIScreenControls:
		details = "Viewing controls"
	}
	env := environmentFor(g.world, g.season, g.weather)
	g.presence.Update(details, fmt.Sprintf("%s · %s", g.world, env.label(g.world, g.season, g.weather)))
}

func drawScene(screen *ebiten.Image, w, h, horizon int, state DriveState, terrain Terrain, world World, env Environment, seed int64, ultra, headlights bool) {
	drawRect(screen, 0, 0, float64(w), float64(horizon), env.Sky)
	drawRect(screen, 0, float64(horizon), float64(w), float64(h-horizon), env.Ground)
	if !env.Night || world == WorldMoon {
		drawCircle(screen, float64(w)*0.79, float64(horizon)*0.48, float64(horizon)*0.075, env.Sun)
	}
	if env.Night {
		drawStars(screen, w, horizon, seed)
	}
	if world != WorldHills {
		drawPlanetHorizon(screen, w, horizon, env, seed)
	}

	// Perspective bands keep the road readable while giving the native build
	// a lightweight, dependency-free 3D-style horizon.
	step := 6
	for y := horizon; y < h; y += step {
		y2 := min(y+step, h)
		depth1 := float64(y-horizon) / float64(h-horizon)
		depth2 := float64(y2-horizon) / float64(h-horizon)
		depth1 *= depth1
		depth2 *= depth2
		scale1 := math.Pow(depth1, 0.67)
		scale2 := math.Pow(depth2, 0.67)

		center1 := roadCenter(w, state.Distance, depth1, terrain, seed)
		center2 := roadCenter(w, state.Distance, depth2, terrain, seed)
		widthScale := terrainWidthScale(terrain)
		half1 := (float64(w)*0.012 + float64(w)*0.43*scale1) * widthScale
		half2 := (float64(w)*0.012 + float64(w)*0.43*scale2) * widthScale

		band := int(math.Floor((float64(y-horizon)+state.Distance*200)/18)) % 2
		grass := env.Ground
		if band == 1 {
			grass = shadeColor(env.Ground, 0.92)
		}
		drawPoly(screen, []point{{0, float64(y)}, {float64(w), float64(y)}, {float64(w), float64(y2)}, {0, float64(y2)}}, grass)

		drawPoly(screen, []point{
			{center1 - half1, float64(y)}, {center1 + half1, float64(y)},
			{center2 + half2, float64(y2)}, {center2 - half2, float64(y2)},
		}, env.Road)

		if band == 0 && scale1 > 0.08 {
			edge1 := math.Max(1, scale1*8)
			edge2 := math.Max(1, scale2*8)
			drawPoly(screen, []point{
				{center1 - half1, float64(y)}, {center1 - half1 + edge1, float64(y)},
				{center2 - half2 + edge2, float64(y2)}, {center2 - half2, float64(y2)},
			}, color.RGBA{231, 229, 218, 255})
			drawPoly(screen, []point{
				{center1 + half1 - edge1, float64(y)}, {center1 + half1, float64(y)},
				{center2 + half2, float64(y2)}, {center2 + half2 - edge2, float64(y2)},
			}, color.RGBA{231, 229, 218, 255})
		}
	}

	drawScenery(screen, w, h, horizon, state, terrain, world, env, seed, ultra)
	drawWeather(screen, w, h, horizon, state.Distance, seed, env)
	if headlights {
		drawPoly(screen, []point{
			{float64(w) * 0.47, float64(h) * 0.82},
			{float64(w) * 0.42, float64(horizon) * 0.65},
			{float64(w) * 0.58, float64(horizon) * 0.65},
			{float64(w) * 0.53, float64(h) * 0.82},
		}, color.RGBA{255, 246, 190, 25})
	}
}

func roadCenter(w int, distance, depth float64, terrain Terrain, seed int64) float64 {
	if terrain == TerrainStraight {
		return float64(w) * 0.5
	}
	amplitude := 0.75
	switch terrain {
	case TerrainEasy, TerrainWide:
		amplitude = 0.6
	case TerrainHard:
		amplitude = 1
	}
	position := distance*9 + depth*3
	curveShape := seededNoise(seed, position)*0.65 + seededNoise(seed+17, position*0.43)*0.35
	curve := curveShape * (1 - depth) * float64(w) * 0.13 * amplitude
	return float64(w)*0.5 + curve
}

func terrainWidthScale(terrain Terrain) float64 {
	switch terrain {
	case TerrainEasy:
		return 3.2 / 3
	case TerrainHard:
		return 2.6 / 3
	case TerrainWide:
		return 3
	case TerrainStraight:
		return 10.0 / 3
	default:
		return 1
	}
}

func roadWidthMeters(terrain Terrain) float64 {
	switch terrain {
	case TerrainEasy:
		return 3.2
	case TerrainHard:
		return 2.6
	case TerrainWide:
		return 9
	case TerrainStraight:
		return 10
	default:
		return 3
	}
}

func drawScenery(screen *ebiten.Image, w, h, horizon int, state DriveState, terrain Terrain, world World, env Environment, seed int64, ultra bool) {
	if world != WorldHills {
		return
	}
	treeCount := 12
	if ultra {
		treeCount = 0
	}
	for i := 0; i < treeCount; i++ {
		depth := float64(i+1) / float64(treeCount+2)
		y := float64(horizon) + depth*depth*float64(h-horizon)
		scale := math.Pow(depth, 1.25)
		road := roadCenter(w, state.Distance, depth*depth, terrain, seed)
		half := (float64(w)*0.012 + float64(w)*0.43*depth*depth) * terrainWidthScale(terrain)
		for side := -1; side <= 1; side += 2 {
			jitter := seededNoise(seed+int64(i*9+side*13), state.Distance*35) * float64(w) * 0.07
			x := road + float64(side)*(half+float64(w)*0.08+scale*float64(w)*0.17) + jitter
			treeH := 14 + scale*115
			drawRect(screen, x-treeH*0.06, y-treeH*0.35, treeH*0.12, treeH*0.38, color.RGBA{89, 58, 38, 255})
			drawPoly(screen, []point{
				{x, y - treeH}, {x - treeH*0.32, y - treeH*0.23}, {x + treeH*0.32, y - treeH*0.23},
			}, shadeColor(env.Foliage, 0.86))
			drawPoly(screen, []point{
				{x, y - treeH*0.77}, {x - treeH*0.26, y - treeH*0.08}, {x + treeH*0.26, y - treeH*0.08},
			}, env.Foliage)
		}
	}
}

func drawPlanetHorizon(screen *ebiten.Image, width, horizon int, env Environment, seed int64) {
	base := color.RGBA{
		R: uint8(float64(env.Ground.R) * 0.72),
		G: uint8(float64(env.Ground.G) * 0.72),
		B: uint8(float64(env.Ground.B) * 0.72),
		A: 255,
	}
	for layer := 0; layer < 2; layer++ {
		points := []point{{0, float64(horizon)}}
		for x := 0; x <= width; x += 48 {
			noise := seededNoise(seed+int64(layer*31), float64(x)/115)
			y := float64(horizon) - 12 - float64(layer*16) - noise*float64(horizon)*0.12
			points = append(points, point{float64(x), y})
		}
		points = append(points, point{float64(width), float64(horizon)}, point{0, float64(horizon)})
		drawPoly(screen, points, shadeColor(base, 0.82+float64(layer)*0.1))
	}
}

func drawStars(screen *ebiten.Image, width, horizon int, seed int64) {
	for i := 0; i < 80; i++ {
		x := seededNoise(seed+int64(i*2), 1.7) * float64(width)
		y := math.Abs(seededNoise(seed+int64(i*3), 9.1)) * float64(horizon) * 0.9
		size := 1.0
		if i%7 == 0 {
			size = 2
		}
		drawCircle(screen, x, y, size, color.RGBA{221, 229, 244, 215})
	}
}

func drawWeather(screen *ebiten.Image, width, height, horizon int, distance float64, seed int64, env Environment) {
	if !env.Rain && !env.Snow {
		return
	}
	for i := 0; i < 75; i++ {
		x := math.Abs(seededNoise(seed+int64(i*5), 12.3)) * float64(width)
		y := float64(horizon) + math.Abs(seededNoise(seed+int64(i*7), 3.7))*float64(height-horizon)
		y += math.Mod(distance*130+float64(i*29), float64(height-horizon))
		y = float64(horizon) + math.Mod(y-float64(horizon), float64(height-horizon))
		if env.Rain {
			drawPoly(screen, []point{{x, y}, {x + 1.5, y - 12}, {x + 2.5, y - 12}, {x + 1, y}}, color.RGBA{197, 217, 232, 130})
		} else {
			drawCircle(screen, x, y, 2.5, color.RGBA{244, 249, 255, 215})
		}
	}
}

func shadeColor(base color.RGBA, amount float64) color.RGBA {
	return color.RGBA{
		R: uint8(math.Max(0, math.Min(255, float64(base.R)*amount))),
		G: uint8(math.Max(0, math.Min(255, float64(base.G)*amount))),
		B: uint8(math.Max(0, math.Min(255, float64(base.B)*amount))),
		A: base.A,
	}
}

func drawHUD(screen *ebiten.Image, g *Game, w, h int) {
	speedUnit, distanceUnit, conversion := "mph", "mi", 1.0
	if g.units == 1 {
		speedUnit, distanceUnit, conversion = "km/h", "km", 1.609344
	}
	speed := int(math.Round(math.Abs(g.state.SpeedMPH) * conversion))
	vehicle := Vehicles[g.vehicle]
	env := environmentFor(g.world, g.season, g.weather)
	label := fmt.Sprintf("%s  |  %s %s  |  %s  |  %d %s", vehicle.Name, g.terrain, g.world, env.label(g.world, g.season, g.weather), speed, speedUnit)
	ebitenutil.DebugPrintAt(screen, label, 18, 18)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Distance %.2f %s | Best %.2f | Crashes %d | Damage %.0f%%", g.state.Distance*conversion, distanceUnit, g.profile.BestDistance*conversion, g.state.Crashes, g.state.Damage), 18, 36)
	render := "Ultra+ off"
	if g.ultra {
		render = "Ultra+ on"
	}
	auto := "manual"
	if g.autoDrive {
		auto = "autodrive"
	}
	cruise := "cruise off"
	if g.cruiseMPH > 0 {
		cruise = fmt.Sprintf("cruise %d %s", int(g.cruiseMPH*conversion), speedUnit)
	}
	drive := []string{"AWD", "FWD", "RWD"}[g.driveMode]
	lights := "lights off"
	if g.headlights {
		lights = "lights on"
	}
	condition := "on road"
	if g.state.OffRoad {
		condition = "off road"
	}
	camera := []string{"Chase", "Chase Far", "First Person"}[g.cameraMode]
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%s | %s | %s | %s | %s", auto, cruise, drive, lights, condition), 18, 54)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%s | %s | W/S or arrows: drive | A/D: steer | Space: handbrake | Shift: boost | 1-5: vehicle | T: terrain | U: Ultra+ | F: autodrive", render, camera), 18, h-42)
	ebitenutil.DebugPrintAt(screen, "Y: world | V: season | G: weather | N: seed | B: drive mode | C: camera | H/L: lights | R: reset | P/ESC: pause", 18, h-24)
}

func drawRect(dst *ebiten.Image, x, y, w, h float64, clr color.RGBA) {
	drawPoly(dst, []point{{x, y}, {x + w, y}, {x + w, y + h}, {x, y + h}}, clr)
}

func drawCircle(dst *ebiten.Image, x, y, radius float64, clr color.RGBA) {
	vertices := make([]point, 20)
	for i := range vertices {
		angle := float64(i) * 2 * math.Pi / float64(len(vertices))
		vertices[i] = point{x + math.Cos(angle)*radius, y + math.Sin(angle)*radius}
	}
	drawPoly(dst, vertices, clr)
}

func drawPoly(dst *ebiten.Image, vertices []point, clr color.RGBA) {
	if len(vertices) < 3 {
		return
	}
	source := whitePixel()
	triangles := make([]ebiten.Vertex, len(vertices))
	for i, p := range vertices {
		triangles[i] = ebiten.Vertex{
			DstX: float32(p.x), DstY: float32(p.y),
			SrcX: 0.5, SrcY: 0.5,
			ColorR: float32(clr.R) / 255, ColorG: float32(clr.G) / 255,
			ColorB: float32(clr.B) / 255, ColorA: float32(clr.A) / 255,
		}
	}
	indices := make([]uint16, 0, (len(vertices)-2)*3)
	for i := 1; i < len(vertices)-1; i++ {
		indices = append(indices, 0, uint16(i), uint16(i+1))
	}
	dst.DrawTriangles(triangles, indices, source, nil)
}

var pixel *ebiten.Image

func whitePixel() *ebiten.Image {
	if pixel == nil {
		pixel = ebiten.NewImage(1, 1)
		pixel.Fill(color.White)
	}
	return pixel
}
