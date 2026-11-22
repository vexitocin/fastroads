package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type Vector2 struct {
	X, Y float64
}

type Vector3 struct {
	X, Y, Z float64
}

type MeshVertex struct {
	Position Vector3
	UV       Vector2
}

type MeshTriangle struct {
	Vertices [3]MeshVertex
}

type Mesh struct {
	Triangles []MeshTriangle
	Min       Vector3
	Max       Vector3
}

func LoadOBJ(path string) (*Mesh, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open OBJ %q: %w", path, err)
	}
	defer file.Close()

	mesh, err := ParseOBJ(file)
	if err != nil {
		return nil, fmt.Errorf("parse OBJ %q: %w", path, err)
	}
	return mesh, nil
}

func ParseOBJ(reader io.Reader) (*Mesh, error) {
	positions := make([]Vector3, 0)
	uvs := make([]Vector2, 0)
	mesh := &Mesh{}
	hasBounds := false
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 4*1024*1024)

	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		switch fields[0] {
		case "v":
			if len(fields) < 4 {
				return nil, fmt.Errorf("line %d: vertex requires three coordinates", lineNumber)
			}
			values, err := parseFloatFields(fields[1:4], lineNumber)
			if err != nil {
				return nil, err
			}
			position := Vector3{values[0], values[1], values[2]}
			positions = append(positions, position)
			if !hasBounds {
				mesh.Min, mesh.Max = position, position
				hasBounds = true
			} else {
				mesh.Min.X = min(mesh.Min.X, position.X)
				mesh.Min.Y = min(mesh.Min.Y, position.Y)
				mesh.Min.Z = min(mesh.Min.Z, position.Z)
				mesh.Max.X = max(mesh.Max.X, position.X)
				mesh.Max.Y = max(mesh.Max.Y, position.Y)
				mesh.Max.Z = max(mesh.Max.Z, position.Z)
			}
		case "vt":
			if len(fields) < 3 {
				return nil, fmt.Errorf("line %d: texture coordinate requires two values", lineNumber)
			}
			values, err := parseFloatFields(fields[1:3], lineNumber)
			if err != nil {
				return nil, err
			}
			uvs = append(uvs, Vector2{values[0], values[1]})
		case "f":
			if len(fields) < 4 {
				return nil, fmt.Errorf("line %d: face requires at least three vertices", lineNumber)
			}
			face := make([]MeshVertex, 0, len(fields)-1)
			for _, token := range fields[1:] {
				vertex, err := parseFaceVertex(token, positions, uvs, lineNumber)
				if err != nil {
					return nil, err
				}
				face = append(face, vertex)
			}
			for i := 1; i < len(face)-1; i++ {
				mesh.Triangles = append(mesh.Triangles, MeshTriangle{
					Vertices: [3]MeshVertex{face[0], face[i], face[i+1]},
				})
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read OBJ: %w", err)
	}
	if len(mesh.Triangles) == 0 {
		return nil, fmt.Errorf("OBJ contains no faces")
	}
	return mesh, nil
}

func parseFloatFields(fields []string, line int) ([]float64, error) {
	values := make([]float64, len(fields))
	for i, field := range fields {
		value, err := strconv.ParseFloat(field, 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid number %q: %w", line, field, err)
		}
		values[i] = value
	}
	return values, nil
}

func parseFaceVertex(token string, positions []Vector3, uvs []Vector2, line int) (MeshVertex, error) {
	parts := strings.Split(token, "/")
	positionIndex, err := resolveOBJIndex(parts[0], len(positions))
	if err != nil {
		return MeshVertex{}, fmt.Errorf("line %d: invalid face vertex %q: %w", line, token, err)
	}
	vertex := MeshVertex{Position: positions[positionIndex]}
	if len(parts) > 1 && parts[1] != "" {
		uvIndex, err := resolveOBJIndex(parts[1], len(uvs))
		if err != nil {
			return MeshVertex{}, fmt.Errorf("line %d: invalid texture index in %q: %w", line, token, err)
		}
		vertex.UV = uvs[uvIndex]
	}
	return vertex, nil
}

func resolveOBJIndex(raw string, count int) (int, error) {
	index, err := strconv.Atoi(raw)
	if err != nil || index == 0 {
		return 0, fmt.Errorf("index %q is not a valid OBJ index", raw)
	}
	if index < 0 {
		index = count + index
	} else {
		index--
	}
	if index < 0 || index >= count {
		return 0, fmt.Errorf("index %q is out of range", raw)
	}
	return index, nil
}
