package palette

import (
	"image/color"
	"math"
	"strconv"
)

// Palette maps a normalized escape value to an opaque RGB color.
type Palette func(float64) color.RGBA

type named struct {
	name  string
	color Palette
}

var catalog = []named{
	{"rgb", hsb(0.9, 0.9, 0.9)},
	{"fire", gradient("000000", "400000", "CC0000", "FF6600", "FFCC00")},
	{"ocean", gradient("000000", "001133", "004488", "0088CC", "AAEFFF", "E6F8FF")},
	{"grayscale", gradient("000000", "404040", "A0A0A0", "FFFFFF")},
	{"sunset", gradient("000000", "1A0033", "660066", "FF3399", "FF9933", "FFCC00")},
	{"neon", hsb(2.5, 1, 1)},
	{"viridis", gradient("440154", "414487", "2A788E", "22A884", "7AD151", "FDE725")},
	{"electric", gradient("000000", "002028", "007864", "00DCB4", "78FFE6")},
}

// Lookup returns a palette by its stable CLI name.
func Lookup(name string) (Palette, bool) {
	for _, entry := range catalog {
		if entry.name == name {
			return entry.color, true
		}
	}
	return nil, false
}

// Names returns catalog names in their display order.
func Names() []string {
	names := make([]string, len(catalog))
	for i, entry := range catalog {
		names[i] = entry.name
	}
	return names
}

func gradient(hexes ...string) Palette {
	stops := make([]color.RGBA, len(hexes))
	for i, hex := range hexes {
		value, _ := strconv.ParseUint(hex, 16, 32)
		stops[i] = color.RGBA{R: uint8(value >> 16), G: uint8(value >> 8), B: uint8(value), A: 255}
	}
	return lut(func(v float64) color.RGBA {
		position := clamp(v) * float64(len(stops)-1)
		index := int(position)
		if index == len(stops)-1 {
			return stops[index]
		}
		fraction := position - float64(index)
		from, to := stops[index], stops[index+1]
		return color.RGBA{
			R: uint8(float64(from.R) + (float64(to.R)-float64(from.R))*fraction),
			G: uint8(float64(from.G) + (float64(to.G)-float64(from.G))*fraction),
			B: uint8(float64(from.B) + (float64(to.B)-float64(from.B))*fraction),
			A: 255,
		}
	})
}

func hsb(cycles, saturation, brightness float64) Palette {
	return lut(func(v float64) color.RGBA {
		r, g, b := hsvToRGB(math.Mod(clamp(v)*cycles, 1), saturation, brightness)
		return color.RGBA{R: uint8(r * 255), G: uint8(g * 255), B: uint8(b * 255), A: 255}
	})
}

func lut(sample Palette) Palette {
	const size = 1024
	colors := make([]color.RGBA, size)
	for i := range colors {
		colors[i] = sample(float64(i) / float64(size-1))
	}
	return func(v float64) color.RGBA { return colors[int(clamp(v)*float64(size-1))] }
}

func hsvToRGB(h, s, v float64) (float64, float64, float64) {
	i := int(h * 6)
	f := h*6 - float64(i)
	p := v * (1 - s)
	q := v * (1 - f*s)
	t := v * (1 - (1-f)*s)
	switch i % 6 {
	case 0:
		return v, t, p
	case 1:
		return q, v, p
	case 2:
		return p, v, t
	case 3:
		return p, q, v
	case 4:
		return t, p, v
	default:
		return v, p, q
	}
}

func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
