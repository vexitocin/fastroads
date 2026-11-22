# Fast Roads - native Go rewrite

This folder contains a native Go reconstruction of the driving game. It does not
run the original browser app. The repository has only a bundled JavaScript build,
so this is a behavior-focused rewrite rather than a line-for-line translation.
The renderer loads vehicle OBJ models and the Roadster texture from the
repository's existing `static/media` folder.

## Run

From this directory:

```sh
go run .
```

The app opens to a menu. Use the buttons, or navigate with Up/Down and Enter.
Settings and Controls are available from the menu; press `P` or `Esc` while
driving to pause. Settings and driving records are saved in the platform's
user configuration directory under `fastroads/recomp.json`.

## Controls

- `W` / Up: accelerate
- `S` / Down: brake or reverse
- `A` / Left and `D` / Right: steer
- `Space`: handbrake
- Left Shift: boost
- `1`-`5`: Roadster, Coach, Lamborghini, Rocket Car, Rocket Coach
- `T`: cycle Easy, Normal, Hard, Wide, and Straight terrain
- `Y`: cycle Hills, Mars, Venus, and Moon
- `V`: cycle the Hills season
- `G`: cycle weather for the selected world and season
- `N`: increment the procedural world seed
- `U`: toggle Ultra+ scenery mode
- `F`: toggle autodrive
- `=`: toggle cruise control
- `B`: cycle drive mode
- `C`: cycle Chase, Chase Far, and First Person cameras
- `H` / `L`: toggle headlights
- `R`: reset
- `P` / `Esc`: pause or resume

The Go build includes seeded road and landscape generation, Hills/Mars/Venus/Moon
environments, seasonal palettes and weather effects, off-road grip and barrier
collisions, damage and distance records, and ambient/vehicle/collision audio.
Settings and cumulative/best distance are persisted locally. Discord Rich
Presence is enabled by default and can be toggled in Settings; Discord must be
running on the same desktop for it to connect. An unavailable Discord client is
reported in the application log and does not prevent the game from running.

This is still a behavior-focused reconstruction, not a byte-for-byte or
line-for-line port. The original source modules and complete simulation engine
are not present in the repository, so some rendering and vehicle dynamics are
approximations.

## GitHub Actions

`.github/workflows/recomp.yml` runs Go tests and builds the native app on Linux,
Windows, and macOS. It does not publish binaries or upload build artifacts.
