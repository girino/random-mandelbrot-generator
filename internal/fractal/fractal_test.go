package fractal

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"os"
	"testing"

	"github.com/girino/mandelbrot-cli/internal/palette"
)

func TestMandelbrotReferencePixels(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 3))
	fire, found := palette.Lookup("fire")
	if !found {
		t.Fatal("fire palette not found")
	}
	Render(img, RenderOptions{
		Center:     complex(-.5, 0),
		ViewWidth:  3.5,
		ViewHeight: 3.5 * 3 / 4,
		Iterations: 16,
		Formula:    Mandelbrot,
		Colorize: func(result Result) color.RGBA {
			if !result.Escaped {
				return color.RGBA{A: 255}
			}
			return fire(result.Value)
		},
		Threads: 1,
		Stderr:  &bytes.Buffer{},
	})
	reference, err := os.Open("testdata/mandelbrot-fire-4x3.ppm")
	if err != nil {
		t.Fatal(err)
	}
	defer reference.Close()
	var magic string
	var width, height, maximum int
	if _, err := fmt.Fscan(reference, &magic, &width, &height, &maximum); err != nil || magic != "P3" || width != 4 || height != 3 || maximum != 255 {
		t.Fatalf("invalid reference header: %q %d %d %d (%v)", magic, width, height, maximum, err)
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			var red, green, blue int
			if _, err := fmt.Fscan(reference, &red, &green, &blue); err != nil {
				t.Fatal(err)
			}
			got := img.RGBAAt(x, y)
			if got.R != uint8(red) || got.G != uint8(green) || got.B != uint8(blue) || got.A != 255 {
				t.Errorf("pixel (%d,%d) = %#v; want RGB(%d,%d,%d)", x, y, got, red, green, blue)
			}
		}
	}
}

func TestFormulaEscapeIsGeneric(t *testing.T) {
	formula := Formula{
		Step:         func(z, c complex128) complex128 { return z + c },
		EscapeRadius: 1,
		Degree:       1,
	}
	result := formula.Escape(complex(2, 0), 4, false)
	if !result.Escaped || result.Value != .25 {
		t.Fatalf("Escape() = %#v; want escaped at one iteration", result)
	}
}

func TestFindBorders(t *testing.T) {
	borders := FindBorders(Mandelbrot, complex(-.5, 0), 3.5, 2, 32, 32)
	if len(borders) == 0 {
		t.Fatal("expected Mandelbrot viewport to contain borders")
	}
	for _, border := range borders {
		if border.Complexity < 1 {
			t.Fatalf("invalid border complexity: %#v", border)
		}
	}
}

func TestBorderCandidatesIncludeInteriorPerimeterAndSeams(t *testing.T) {
	interior := []bool{
		true, true, true,
		true, false, true,
		true, true, true,
	}
	candidates := borderCandidates(interior, 3, 3)
	if len(candidates) != 8 {
		t.Fatalf("got %d candidates; want all eight interior perimeter pixels", len(candidates))
	}
}

func TestAndroidProgressiveIterations(t *testing.T) {
	// A 320px Android viewport has reference zoom 1.025390625 and resolves to base 40.
	if got := AndroidProgressiveIterations(1.025390625, 320); got != 40 {
		t.Fatalf("reference iterations = %d; want 40", got)
	}
	if AndroidProgressiveIterations(1e100, 320) != 1_048_576 {
		t.Fatal("expected progressive iterations to cap")
	}
}
