package main

import (
	"image/color"
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
)

type projectedTriangle struct {
	depth    float64
	vertices [3]ebiten.Vertex
}

func drawVehicle(dst *ebiten.Image, width, height int, vehicle Vehicle, assets VehicleAssets, lateral, steer float64, variant, cameraMode int) {
	if cameraMode == 2 {
		return
	}
	drawVehicleWheels(dst, width, height, vehicle, lateral, steer, cameraMode)
	if assets.Body == nil {
		return
	}

	bodyLength := assets.Body.Max.X - assets.Body.Min.X
	if bodyLength <= 0 {
		return
	}
	scale := vehicle.Length / bodyLength
	yaw := steer * 0.3
	sinYaw, cosYaw := math.Sin(yaw), math.Cos(yaw)
	focal := float64(height) * 0.78
	centerX, centerY := float64(width)*0.5, float64(height)*0.82
	cameraY, cameraZ := 0.7, 6.5
	if cameraMode == 1 {
		cameraZ = 10
	}

	tint := color.RGBA{230, 230, 230, 255}
	switch variant {
	case 2:
		tint = color.RGBA{236, 196, 50, 255}
	case 3:
		tint = color.RGBA{244, 246, 250, 255}
	case 4:
		tint = color.RGBA{54, 120, 194, 255}
	}
	if assets.Texture != nil {
		tint = color.RGBA{255, 255, 255, 255}
	}

	rendered := make([]projectedTriangle, 0, len(assets.Body.Triangles))
	for _, triangle := range assets.Body.Triangles {
		var projected projectedTriangle
		visible := true
		for i, vertex := range triangle.Vertices {
			position := vertex.Position
			localX := position.Z - (assets.Body.Min.Z+assets.Body.Max.Z)*0.5
			localY := position.Y - assets.Body.Min.Y
			localZ := position.X - (assets.Body.Min.X+assets.Body.Max.X)*0.5
			worldX := lateral + localX*scale*cosYaw + localZ*scale*sinYaw
			worldY := localY * scale
			worldZ := localZ*scale*cosYaw - localX*scale*sinYaw
			depth := cameraZ + worldZ
			if depth <= 0.1 {
				visible = false
				break
			}
			x := centerX + worldX*focal/depth
			y := centerY - (worldY-cameraY)*focal/depth
			projected.depth += depth / 3

			vertexColor := tint
			if assets.Texture == nil {
				shade := triangleShade(triangle)
				vertexColor.R = uint8(float64(tint.R) * shade)
				vertexColor.G = uint8(float64(tint.G) * shade)
				vertexColor.B = uint8(float64(tint.B) * shade)
			}
			var sourceX, sourceY float32
			if assets.Texture != nil {
				sourceX = float32(vertex.UV.X * float64(assets.Texture.Bounds().Dx()))
				sourceY = float32((1 - vertex.UV.Y) * float64(assets.Texture.Bounds().Dy()))
			} else {
				sourceX, sourceY = 0.5, 0.5
			}
			projected.vertices[i] = ebiten.Vertex{
				DstX: float32(x), DstY: float32(y),
				SrcX: sourceX, SrcY: sourceY,
				ColorR: float32(vertexColor.R) / 255,
				ColorG: float32(vertexColor.G) / 255,
				ColorB: float32(vertexColor.B) / 255,
				ColorA: float32(vertexColor.A) / 255,
			}
		}
		if visible {
			rendered = append(rendered, projected)
		}
	}
	sort.SliceStable(rendered, func(i, j int) bool {
		return rendered[i].depth > rendered[j].depth
	})

	vertices := make([]ebiten.Vertex, 0, len(rendered)*3)
	indices := make([]uint16, 0, len(rendered)*3)
	for _, triangle := range rendered {
		base := uint16(len(vertices))
		vertices = append(vertices, triangle.vertices[:]...)
		indices = append(indices, base, base+1, base+2)
	}
	if len(vertices) == 0 {
		return
	}
	texture := assets.Texture
	if texture == nil {
		texture = whitePixel()
	}
	dst.DrawTriangles(vertices, indices, texture, nil)
}

func triangleShade(triangle MeshTriangle) float64 {
	a := triangle.Vertices[0].Position
	b := triangle.Vertices[1].Position
	c := triangle.Vertices[2].Position
	ab := Vector3{b.X - a.X, b.Y - a.Y, b.Z - a.Z}
	ac := Vector3{c.X - a.X, c.Y - a.Y, c.Z - a.Z}
	normal := Vector3{
		X: ab.Y*ac.Z - ab.Z*ac.Y,
		Y: ab.Z*ac.X - ab.X*ac.Z,
		Z: ab.X*ac.Y - ab.Y*ac.X,
	}
	length := math.Sqrt(normal.X*normal.X + normal.Y*normal.Y + normal.Z*normal.Z)
	if length == 0 {
		return 0.55
	}
	light := Vector3{X: 0.3, Y: 0.85, Z: -0.5}
	dot := (normal.X*light.X + normal.Y*light.Y + normal.Z*light.Z) / length
	return 0.45 + 0.55*math.Max(0, dot)
}

func drawVehicleWheels(dst *ebiten.Image, width, height int, vehicle Vehicle, lateral, steer float64, cameraMode int) {
	focal := float64(height) * 0.78
	centerX, centerY := float64(width)*0.5, float64(height)*0.82
	yaw := steer * 0.3
	sinYaw, cosYaw := math.Sin(yaw), math.Cos(yaw)
	for _, side := range []float64{-1, 1} {
		for _, axle := range []float64{-0.34, 0.34} {
			localX := side * vehicle.Width * 0.5
			localZ := axle * vehicle.Length
			worldX := lateral + localX*cosYaw + localZ*sinYaw
			worldZ := localZ*cosYaw - localX*sinYaw
			cameraZ := 6.5
			if cameraMode == 1 {
				cameraZ = 10
			}
			depth := cameraZ + worldZ
			if depth <= 0.1 {
				continue
			}
			x := centerX + worldX*focal/depth
			y := centerY - (vehicle.WheelRadius-0.7)*focal/depth
			radius := math.Max(2, vehicle.WheelRadius*focal/depth)
			points := make([]point, 16)
			for i := range points {
				angle := float64(i) * 2 * math.Pi / float64(len(points))
				points[i] = point{x + math.Cos(angle)*radius*0.66, y + math.Sin(angle)*radius}
			}
			drawPoly(dst, points, color.RGBA{26, 27, 29, 255})
		}
	}
}
