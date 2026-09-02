package fractal

import "math"

// Border is a mixed interior/exterior sample location relative to a viewport center.
type Border struct {
	RealOffset      float64
	ImaginaryOffset float64
	Complexity      int
}

// FindBorders samples a viewport and returns interior/exterior transition points.
// SampleSize controls the horizontal resolution; the vertical resolution preserves
// the viewport aspect ratio so samples remain approximately square.
func FindBorders(formula Formula, center complex128, viewWidth, viewHeight float64, sampleSize, iterations int) []Border {
	if sampleSize < 3 || iterations < 1 || viewWidth <= 0 || viewHeight <= 0 {
		return nil
	}
	width := sampleSize
	height := max(3, int(math.Round(float64(sampleSize)*viewHeight/viewWidth)))
	interior := make([]bool, width*height)
	for y := range height {
		imaginary := imag(center) + (0.5-float64(y)/float64(height-1))*viewHeight
		for x := range width {
			real := real(center) + (float64(x)/float64(width-1)-0.5)*viewWidth
			interior[y*width+x] = !formula.Escape(complex(real, imaginary), iterations, false).Escaped
		}
	}

	borders := make([]Border, 0)
	for y := 1; y < height-1; y++ {
		for x := 1; x < width-1; x++ {
			current := interior[y*width+x]
			complexity := 0
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if (dx != 0 || dy != 0) && interior[(y+dy)*width+x+dx] != current {
						complexity++
					}
				}
			}
			if complexity > 0 {
				borders = append(borders, Border{
					RealOffset:      (float64(x)/float64(width-1) - 0.5) * viewWidth,
					ImaginaryOffset: (0.5 - float64(y)/float64(height-1)) * viewHeight,
					Complexity:      complexity,
				})
			}
		}
	}
	return borders
}
