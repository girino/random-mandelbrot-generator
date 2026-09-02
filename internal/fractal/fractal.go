package fractal

import (
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
	"sync"
)

// Step advances z once for a complex parameter c.
type Step func(z, c complex128) complex128

// Formula defines an escape-time fractal independent of its color palette.
type Formula struct {
	Step         Step
	EscapeRadius float64
	Degree       float64
}

// Result is the normalized output of a generic escape calculation.
type Result struct {
	Escaped bool
	Value   float64
}

// Mandelbrot is the quadratic z = z² + c formula used by the MVP.
var Mandelbrot = Formula{
	Step:         func(z, c complex128) complex128 { return z*z + c },
	EscapeRadius: 4,
	Degree:       2,
}

// Escape iterates a formula and returns a palette-independent result.
func (formula Formula) Escape(c complex128, maxIterations int, smooth bool) Result {
	z := complex(0.0, 0.0)
	radiusSquared := formula.EscapeRadius * formula.EscapeRadius
	for n := 0; n < maxIterations; n++ {
		z = formula.Step(z, c)
		if real(z)*real(z)+imag(z)*imag(z) > radiusSquared {
			value := float64(n+1) / float64(maxIterations)
			if smooth && formula.Degree > 1 {
				value = (float64(n+1) - math.Log(math.Log(math.Hypot(real(z), imag(z))))/math.Log(formula.Degree)) / float64(maxIterations)
			}
			return Result{Escaped: true, Value: clamp(value)}
		}
	}
	return Result{}
}

// RenderOptions separates generic fractal evaluation from a color mapping.
type RenderOptions struct {
	Center     complex128
	ViewWidth  float64
	ViewHeight float64
	Iterations int
	Smooth     bool
	Formula    Formula
	Colorize   func(Result) color.RGBA
	Threads    int
	Progress   bool
	Stderr     io.Writer
	Adaptive   *AdaptiveOptions
}

// AdaptiveOptions controls Android-style boundary refinement after the first pass.
type AdaptiveOptions struct {
	MaxIterations int
	MaxRounds     int
}

// Render fills img using the configured escape-time formula and colorizer.
func Render(img *image.RGBA, options RenderOptions) {
	width, height := img.Bounds().Dx(), img.Bounds().Dy()
	var interior []bool
	if options.Adaptive != nil {
		interior = make([]bool, width*height)
	}
	renderPass(img, options, options.Iterations, interior)
	if options.Adaptive != nil {
		refineAdaptive(img, options, interior)
	}
}

func renderPass(img *image.RGBA, options RenderOptions, iterations int, interior []bool) {
	width, height := img.Bounds().Dx(), img.Bounds().Dy()
	jobs := make(chan int)
	var workers sync.WaitGroup
	var progressMu sync.Mutex
	completed := 0
	for range options.Threads {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for y := range jobs {
				yRatio := 0.5
				if height > 1 {
					yRatio = float64(y) / float64(height-1)
				}
				imaginary := imag(options.Center) + (0.5-yRatio)*options.ViewHeight
				for x := range width {
					xRatio := 0.5
					if width > 1 {
						xRatio = float64(x) / float64(width-1)
					}
					real := real(options.Center) + (xRatio-0.5)*options.ViewWidth
					result := options.Formula.Escape(complex(real, imaginary), iterations, options.Smooth)
					img.SetRGBA(x, y, options.Colorize(result))
					if interior != nil {
						interior[y*width+x] = !result.Escaped
					}
				}
				if options.Progress {
					progressMu.Lock()
					completed++
					fmt.Fprintf(options.Stderr, "progress: %d/%d rows\n", completed, height)
					progressMu.Unlock()
				}
			}
		}()
	}
	for y := range height {
		jobs <- y
	}
	close(jobs)
	workers.Wait()
}

func refineAdaptive(img *image.RGBA, options RenderOptions, interior []bool) {
	maxIterations := max(options.Adaptive.MaxIterations, options.Iterations)
	maxRounds := options.Adaptive.MaxRounds
	if maxRounds < 1 {
		maxRounds = 18
	}
	current := options.Iterations
	for round := 0; round < maxRounds && current < maxIterations; round++ {
		next := maxIterations
		if current <= maxIterations/2 {
			next = current * 2
		}
		escapedAtLimit := false
		visitedAtLimit := make([]bool, len(interior))
		for {
			candidates := borderCandidates(interior, img.Bounds().Dx(), img.Bounds().Dy())
			if len(candidates) == 0 {
				return
			}
			escapedThisPass := false
			testedThisPass := false
			for _, index := range candidates {
				if visitedAtLimit[index] {
					continue
				}
				visitedAtLimit[index] = true
				testedThisPass = true
				x, y := index%img.Bounds().Dx(), index/img.Bounds().Dx()
				point := viewportPoint(options, x, y, img.Bounds().Dx(), img.Bounds().Dy())
				result := options.Formula.Escape(point, next, options.Smooth)
				if result.Escaped {
					img.SetRGBA(x, y, options.Colorize(result))
					interior[index] = false
					escapedThisPass = true
					escapedAtLimit = true
				}
			}
			if !testedThisPass || !escapedThisPass {
				break
			}
		}
		if !escapedAtLimit {
			return
		}
		current = next
	}
}

func borderCandidates(interior []bool, width, height int) []int {
	candidates := make([]int, 0)
	for y := range height {
		for x := range width {
			index := y*width + x
			if !interior[index] {
				continue
			}
			if x == 0 || y == 0 || x == width-1 || y == height-1 ||
				!interior[index-1] || !interior[index+1] || !interior[index-width] || !interior[index+width] {
				candidates = append(candidates, index)
			}
		}
	}
	return candidates
}

func viewportPoint(options RenderOptions, x, y, width, height int) complex128 {
	xRatio, yRatio := 0.5, 0.5
	if width > 1 {
		xRatio = float64(x) / float64(width-1)
	}
	if height > 1 {
		yRatio = float64(y) / float64(height-1)
	}
	return complex(
		real(options.Center)+(xRatio-0.5)*options.ViewWidth,
		imag(options.Center)+(0.5-yRatio)*options.ViewHeight,
	)
}

func clamp(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
