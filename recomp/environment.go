package main

import (
	"fmt"
	"image/color"
	"math"
)

type World uint8

const (
	WorldHills World = iota
	WorldMars
	WorldVenus
	WorldMoon
	worldCount
)

func (w World) String() string {
	switch w {
	case WorldMars:
		return "Mars"
	case WorldVenus:
		return "Venus"
	case WorldMoon:
		return "Moon"
	default:
		return "Hills"
	}
}

type Season uint8

const (
	SeasonSummer Season = iota
	SeasonSpring
	SeasonAutumn
	SeasonWinter
	seasonCount
)

func (s Season) String() string {
	return [...]string{"Summer", "Spring", "Autumn", "Winter"}[s%seasonCount]
}

type Weather uint8

type Environment struct {
	Sky          color.RGBA
	Ground       color.RGBA
	Road         color.RGBA
	Foliage      color.RGBA
	Sun          color.RGBA
	Night        bool
	Rain         bool
	Snow         bool
	Headlights   bool
	OffRoadGrip  float64
	AmbientAsset string
}

func weatherOptions(world World, season Season) []string {
	switch world {
	case WorldMars:
		return []string{"Day", "Dawn", "Night"}
	case WorldVenus:
		return []string{"Day", "Sunset"}
	case WorldMoon:
		return []string{"Day"}
	default:
		switch season {
		case SeasonSpring:
			return []string{"Clear", "Sunrise", "Rain", "Sunset", "Night"}
		case SeasonAutumn:
			return []string{"Clear", "Sunrise", "Rain", "Twilight", "Night"}
		case SeasonWinter:
			return []string{"Clear", "Sunrise", "Snow", "Sunset", "Night"}
		default:
			return []string{"Clear", "Sunrise", "Rain", "Sunset", "Night"}
		}
	}
}

func environmentFor(world World, season Season, weather Weather) Environment {
	options := weatherOptions(world, season)
	name := options[int(weather)%len(options)]
	env := Environment{
		Sky:          color.RGBA{113, 190, 235, 255},
		Ground:       color.RGBA{88, 142, 76, 255},
		Road:         color.RGBA{70, 72, 76, 255},
		Foliage:      color.RGBA{45, 114, 59, 255},
		Sun:          color.RGBA{255, 231, 153, 255},
		OffRoadGrip:  0.75,
		AmbientAsset: "ambiance_summer_low.2130513f.mp3",
	}

	switch season {
	case SeasonSpring:
		env.Ground = color.RGBA{111, 157, 88, 255}
		env.Foliage = color.RGBA{66, 145, 75, 255}
		env.AmbientAsset = "ambiance_summer_low.2130513f.mp3"
	case SeasonAutumn:
		env.Ground = color.RGBA{137, 111, 73, 255}
		env.Foliage = color.RGBA{172, 91, 39, 255}
	case SeasonWinter:
		env.Ground = color.RGBA{210, 218, 222, 255}
		env.Foliage = color.RGBA{105, 121, 112, 255}
		env.Road = color.RGBA{92, 96, 102, 255}
		env.OffRoadGrip = 0.45
	}

	if world != WorldHills {
		env.AmbientAsset = "ambiance_offworld.41a010f6.mp3"
		env.Foliage = color.RGBA{91, 77, 67, 255}
		switch world {
		case WorldMars:
			env.Sky, env.Ground = color.RGBA{204, 115, 82, 255}, color.RGBA{153, 87, 67, 255}
		case WorldVenus:
			env.Sky, env.Ground = color.RGBA{225, 175, 139, 255}, color.RGBA{152, 103, 76, 255}
			env.Foliage = color.RGBA{138, 85, 59, 255}
		case WorldMoon:
			env.Sky, env.Ground = color.RGBA{17, 18, 26, 255}, color.RGBA{84, 83, 87, 255}
			env.Road = color.RGBA{61, 62, 67, 255}
			env.Night, env.Headlights = true, true
			env.Sun = color.RGBA{220, 223, 232, 255}
		}
	}

	switch name {
	case "Sunrise":
		env.Sky = color.RGBA{239, 154, 127, 255}
		env.Sun = color.RGBA{255, 205, 157, 255}
	case "Sunset":
		env.Sky = color.RGBA{190, 104, 120, 255}
		env.Sun = color.RGBA{255, 173, 120, 255}
		if world == WorldVenus {
			env.Sky = color.RGBA{90, 68, 114, 255}
			env.Night, env.Headlights = true, true
		}
	case "Twilight":
		env.Sky = color.RGBA{88, 81, 132, 255}
		env.Sun = color.RGBA{177, 152, 205, 255}
	case "Rain":
		env.Sky = color.RGBA{106, 121, 137, 255}
		env.Ground = color.RGBA{70, 100, 79, 255}
		env.Road = color.RGBA{57, 64, 72, 255}
		env.OffRoadGrip = 0.6
		env.Rain, env.AmbientAsset = true, "ambiance_cloudy.4d5b0a33.mp3"
		if world != WorldHills {
			env.AmbientAsset = "ambiance_offworld.41a010f6.mp3"
		}
	case "Snow":
		env.Sky = color.RGBA{174, 193, 208, 255}
		env.Ground = color.RGBA{220, 226, 230, 255}
		env.Road = color.RGBA{104, 111, 119, 255}
		env.Snow, env.OffRoadGrip = true, 0.35
	case "Night":
		env.Sky = color.RGBA{18, 23, 39, 255}
		env.Ground = color.RGBA{42, 57, 47, 255}
		env.Road = color.RGBA{42, 44, 52, 255}
		env.Night, env.Headlights = true, true
		env.Sun = color.RGBA{199, 207, 230, 255}
	case "Day":
		if world == WorldMars {
			env.Sky = color.RGBA{195, 106, 76, 255}
		}
	case "Dawn":
		env.Sky = color.RGBA{85, 73, 113, 255}
		env.Ground = color.RGBA{91, 72, 68, 255}
		env.Night, env.Headlights = true, true
		if world == WorldVenus {
			env.Sky = color.RGBA{90, 68, 114, 255}
			env.Night, env.Headlights = true, true
		}
	}

	return env
}

func (e Environment) label(world World, season Season, weather Weather) string {
	options := weatherOptions(world, season)
	name := options[int(weather)%len(options)]
	if world == WorldHills {
		return fmt.Sprintf("%s %s", season, name)
	}
	return name
}

func seededNoise(seed int64, x float64) float64 {
	left := math.Floor(x)
	t := x - left
	t = t * t * (3 - 2*t)
	a := latticeNoise(seed, int64(left))
	b := latticeNoise(seed, int64(left)+1)
	return a + (b-a)*t
}

func latticeNoise(seed, position int64) float64 {
	value := uint64(seed) ^ uint64(position)*0x9e3779b97f4a7c15
	value ^= value >> 30
	value *= 0xbf58476d1ce4e5b9
	value ^= value >> 27
	value *= 0x94d049bb133111eb
	value ^= value >> 31
	return float64(value>>11)/float64(uint64(1)<<53)*2 - 1
}
