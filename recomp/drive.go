package main

import (
	"math"
)

type Terrain uint8

const (
	TerrainEasy Terrain = iota
	TerrainNormal
	TerrainHard
	TerrainWide
	TerrainStraight
	terrainCount
)

func (t Terrain) String() string {
	switch t {
	case TerrainEasy:
		return "Easy"
	case TerrainHard:
		return "Hard"
	case TerrainWide:
		return "Wide"
	case TerrainStraight:
		return "Straight"
	default:
		return "Normal"
	}
}

type Vehicle struct {
	Name         string
	TopSpeedMPH  float64
	ReverseMPH   float64
	Acceleration float64
	Brake        float64
	Steering     float64
	Jerk         float64
	Drag         float64
	Grip         float64
	Width        float64
	Length       float64
	WheelRadius  float64
	BodyAsset    string
	TextureAsset string
}

const metresPerSecondToMPH = 2.2369362921

func vehicle(name string, topSpeed, reverse, accel, brake, steer, jerk, drag, grip, width, length, wheelRadius float64, body, texture string) Vehicle {
	return Vehicle{
		Name:         name,
		TopSpeedMPH:  topSpeed * metresPerSecondToMPH,
		ReverseMPH:   reverse * metresPerSecondToMPH,
		Acceleration: accel * metresPerSecondToMPH,
		Brake:        brake * metresPerSecondToMPH,
		Steering:     steer,
		Jerk:         jerk * metresPerSecondToMPH,
		Drag:         drag,
		Grip:         grip,
		Width:        width,
		Length:       length,
		WheelRadius:  wheelRadius,
		BodyAsset:    body,
		TextureAsset: texture,
	}
}

var Vehicles = []Vehicle{
	vehicle("Roadster", 45, 5, 9, 8, 0.68, 48, 0.001, 0.1, 1.36, 2.75, 0.342665,
		"roadster-07.0d45cf64.obj", "roadster-07b_map.17a06a45.jpg"),
	vehicle("Coach", 32, 5, 6, 4, 0.65, 6, 0.002, 1, 2.24, 5.47, 0.3955,
		"bus-02.07c5f9f2.obj", ""),
	vehicle("Lamborghini", 500, 60, 40, 50, 0.68, 60, 0.001, 0.1, 1.36, 2.75, 0.342665,
		"roadster-07.0d45cf64.obj", "roadster-07b_map.17a06a45.jpg"),
	{
		Name: "Rocket Car", TopSpeedMPH: 60000, ReverseMPH: 60000,
		Acceleration: 60000, Brake: 60000, Steering: 0.68, Jerk: 80 * metresPerSecondToMPH,
		Drag: 0.0001, Grip: 0.1, Width: 1.36, Length: 2.75, WheelRadius: 0.342665,
		BodyAsset: "roadster-07.0d45cf64.obj", TextureAsset: "roadster-07b_map.17a06a45.jpg",
	},
	{
		Name: "Rocket Coach", TopSpeedMPH: 30000, ReverseMPH: 30000,
		Acceleration: 30000, Brake: 30000, Steering: 0.65, Jerk: 6 * metresPerSecondToMPH, Drag: 0.002, Grip: 1,
		Width: 2.24, Length: 5.47, WheelRadius: 0.3955, BodyAsset: "bus-02.07c5f9f2.obj",
	},
}

type DriveState struct {
	SpeedMPH      float64
	Distance      float64
	Steer         float64
	Lateral       float64
	Drive         float64
	Damage        float64
	CrashCooldown float64
	Crashes       uint64
	OffRoad       bool
}

type DriveInput struct {
	Accelerate  bool
	Reverse     bool
	Handbrake   bool
	SteerLeft   bool
	SteerRight  bool
	Boost       bool
	AutoDrive   bool
	CruiseMPH   float64
	DriveMode   int
	RoadWidth   float64
	OffRoadGrip float64
}

func StepDrive(state DriveState, vehicle Vehicle, input DriveInput, dt float64) DriveState {
	if dt <= 0 || math.IsNaN(dt) || math.IsInf(dt, 0) {
		return state
	}
	state.CrashCooldown = math.Max(0, state.CrashCooldown-dt)
	if input.CruiseMPH > 0 && !input.Reverse && !input.Handbrake {
		input.Accelerate = state.SpeedMPH < input.CruiseMPH
	}
	if input.AutoDrive && input.CruiseMPH > 0 {
		input.Accelerate = state.SpeedMPH < input.CruiseMPH
		input.Reverse = false
	}

	targetDrive := 0.0
	braking := false
	if input.Accelerate && state.SpeedMPH < 0 {
		braking = true
	} else if input.Accelerate {
		boost := 1.0
		if input.Boost {
			boost += math.Max(0, 1-math.Abs(state.SpeedMPH)/vehicle.TopSpeedMPH)
			if input.DriveMode == 0 {
				boost *= 1.1
			} else {
				boost *= 0.9
			}
		}
		targetDrive = vehicle.Acceleration * boost
	} else if input.Reverse && state.SpeedMPH > 0 {
		braking = true
	} else if input.Reverse {
		targetDrive = -vehicle.Acceleration
	}

	if braking {
		state.SpeedMPH = approachZero(state.SpeedMPH, vehicle.Brake*dt)
		state.Drive = 0
	} else {
		state.Drive = moveTowards(state.Drive, targetDrive, vehicle.Jerk*dt)
		state.SpeedMPH += state.Drive * dt
	}
	if targetDrive == 0 && !braking {
		resistance := vehicle.Drag*state.SpeedMPH*math.Abs(state.SpeedMPH) + 4*math.Copysign(1, state.SpeedMPH)
		state.SpeedMPH = approachZero(state.SpeedMPH, math.Abs(resistance*dt))
	}
	if input.Handbrake {
		state.SpeedMPH = approachZero(state.SpeedMPH, vehicle.Brake*1.5*dt)
		state.Drive = 0
	}
	state.SpeedMPH = math.Max(-vehicle.ReverseMPH, math.Min(vehicle.TopSpeedMPH, state.SpeedMPH))

	steer := 0.0
	if input.SteerLeft {
		steer--
	}
	if input.SteerRight {
		steer++
	}
	state.Steer += steer * vehicle.Steering * dt * 2
	state.Steer *= math.Pow(0.08, dt)
	state.Steer = math.Max(-1, math.Min(1, state.Steer))

	speedFactor := math.Min(1, math.Abs(state.SpeedMPH)/(vehicle.TopSpeedMPH*0.25))
	grip := vehicle.Grip
	if input.DriveMode == 1 {
		grip *= 0.9
	} else if input.DriveMode == 2 {
		grip *= 1.1
	}
	if input.Handbrake {
		grip *= 0.25
	}
	state.Lateral += state.Steer * vehicle.Steering * speedFactor * grip * dt * 2
	if input.AutoDrive && !input.SteerLeft && !input.SteerRight {
		state.Lateral *= math.Pow(0.2, dt)
	}
	state.Lateral *= math.Pow(0.9, dt)
	state.Lateral = math.Max(-8, math.Min(8, state.Lateral))
	state.OffRoad = false
	if input.RoadWidth > 0 {
		laneLimit := math.Max(0, (input.RoadWidth-vehicle.Width)/2)
		state.OffRoad = math.Abs(state.Lateral) > laneLimit
		if state.OffRoad {
			grip := input.OffRoadGrip
			if grip <= 0 || grip > 1 {
				grip = 0.5
			}
			state.SpeedMPH = approachZero(state.SpeedMPH, vehicle.Brake*(1-grip)*dt)
			barrier := input.RoadWidth/2 + vehicle.Width*0.4
			if math.Abs(state.Lateral) > barrier && state.CrashCooldown == 0 {
				state.SpeedMPH *= 0.3
				state.Damage = math.Min(100, state.Damage+12)
				state.Crashes++
				state.CrashCooldown = 1.25
				state.Lateral = math.Copysign(barrier, state.Lateral)
			}
		}
	}
	state.Distance += math.Abs(state.SpeedMPH) * dt / 3600
	return state
}

func moveTowards(current, target, step float64) float64 {
	if current < target {
		return math.Min(current+step, target)
	}
	return math.Max(current-step, target)
}

func approachZero(value, amount float64) float64 {
	if math.Abs(value) <= amount {
		return 0
	}
	return value - math.Copysign(amount, value)
}

type point struct {
	x, y float64
}
