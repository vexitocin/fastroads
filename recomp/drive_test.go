package main

import (
	"math"
	"testing"
)

func TestStepDriveAcceleratesAndCapsAtVehicleLimit(t *testing.T) {
	state := DriveState{}
	for i := 0; i < 1000; i++ {
		state = StepDrive(state, Vehicles[0], DriveInput{Accelerate: true}, 1.0/60)
	}
	if state.SpeedMPH <= 0 {
		t.Fatal("accelerating should increase speed")
	}
	if state.SpeedMPH > Vehicles[0].TopSpeedMPH {
		t.Fatalf("speed exceeded vehicle limit: %f", state.SpeedMPH)
	}
	if state.Distance <= 0 {
		t.Fatal("driving should increase distance")
	}
}

func TestStepDriveBrakesWithoutGoingNegative(t *testing.T) {
	state := DriveState{SpeedMPH: 4}
	state = StepDrive(state, Vehicles[0], DriveInput{Reverse: true}, 1)
	if state.SpeedMPH != 0 {
		t.Fatalf("expected speed to clamp to zero, got %f", state.SpeedMPH)
	}
}

func TestStepDriveReverseIsCappedAndDistanceAlwaysIncreases(t *testing.T) {
	state := DriveState{}
	for i := 0; i < 1000; i++ {
		state = StepDrive(state, Vehicles[0], DriveInput{Reverse: true}, 1.0/60)
	}
	if state.SpeedMPH < -Vehicles[0].ReverseMPH {
		t.Fatalf("reverse speed exceeded vehicle limit: %f", state.SpeedMPH)
	}
	if state.Distance <= 0 {
		t.Fatal("reverse driving should increase distance")
	}
}

func TestStepDriveBoostAcceleratesFaster(t *testing.T) {
	start := DriveState{Drive: Vehicles[0].Acceleration}
	base := StepDrive(start, Vehicles[0], DriveInput{Accelerate: true}, 0.1)
	boosted := StepDrive(start, Vehicles[0], DriveInput{Accelerate: true, Boost: true}, 0.1)
	if boosted.SpeedMPH <= base.SpeedMPH {
		t.Fatalf("boost should increase acceleration: normal %f, boosted %f", base.SpeedMPH, boosted.SpeedMPH)
	}
}

func TestStepDriveThrottleRampsByVehicleJerk(t *testing.T) {
	state := StepDrive(DriveState{}, Vehicles[0], DriveInput{Accelerate: true}, 0.1)
	maxDrive := Vehicles[0].Jerk * 0.1
	if state.Drive > maxDrive {
		t.Fatalf("drive force exceeded jerk ramp: got %f, max %f", state.Drive, maxDrive)
	}
}

func TestStepDriveSteeringIsBoundedAndRecenters(t *testing.T) {
	state := DriveState{}
	for i := 0; i < 1000; i++ {
		state = StepDrive(state, Vehicles[0], DriveInput{SteerRight: true}, 1.0/60)
	}
	if state.Steer > 1 || state.Steer < -1 {
		t.Fatalf("steering escaped bounds: %f", state.Steer)
	}
	state = StepDrive(state, Vehicles[0], DriveInput{}, 1.5)
	if math.Abs(state.Steer) > 0.04 {
		t.Fatalf("steering did not recenter: %f", state.Steer)
	}
}

func TestStepDriveInvalidDeltaDoesNotChangeState(t *testing.T) {
	before := DriveState{SpeedMPH: 42, Distance: 1.25, Steer: -0.5}
	got := StepDrive(before, Vehicles[1], DriveInput{Accelerate: true}, 0)
	if got != before {
		t.Fatalf("invalid timestep changed state: got %+v, want %+v", got, before)
	}
}

func TestStepDriveOffRoadCollisionReducesSpeedAndRecordsDamage(t *testing.T) {
	state := DriveState{SpeedMPH: 80, Lateral: 2.8}
	got := StepDrive(state, Vehicles[0], DriveInput{RoadWidth: 3, OffRoadGrip: 0.6}, 1.0/60)
	if !got.OffRoad {
		t.Fatal("expected vehicle to be off road")
	}
	if got.Crashes != 1 || got.Damage == 0 {
		t.Fatalf("collision was not recorded: crashes=%d damage=%f", got.Crashes, got.Damage)
	}
	if got.SpeedMPH >= state.SpeedMPH {
		t.Fatalf("collision should reduce speed: before %f, after %f", state.SpeedMPH, got.SpeedMPH)
	}
	if got.Lateral > 3 || got.Lateral < -3 {
		t.Fatalf("collision did not constrain lateral position: %f", got.Lateral)
	}
}
