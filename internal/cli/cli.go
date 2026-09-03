package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/girino/random-mandelbrot-generator/internal/fractal"
	"github.com/girino/random-mandelbrot-generator/internal/palette"
)

const (
	defaultWidth        = 1024
	defaultHeight       = 1024
	defaultIterations   = 64
	defaultAdaptiveCap  = 262_144
	defaultAdaptiveRuns = 18
	safeSide            = 16384
	safePixels          = 100_000_000
	safeIterations      = 1_048_576
	defaultViewportWide = 3.5
)

type renderOptions struct {
	output        string
	outputDir     string
	width         int
	height        int
	size          string
	center        string
	zoom          float64
	iterations    int
	maxIterations int
	adaptive      bool
	palette       string
	smooth        bool
	threads       int
	force         bool
	unsafe        bool
	quiet         bool
	progress      bool
}

type randomOptions struct {
	output        string
	outputDir     string
	width         int
	height        int
	size          string
	iterations    int
	maxIterations int
	adaptive      bool
	palette       string
	randomPalette bool
	smooth        bool
	threads       int
	force         bool
	unsafe        bool
	quiet         bool
	progress      bool
	seed          int64
	minPasses     int
	maxPasses     int
	minZoom       float64
	maxZoom       float64
	sampleSize    int
	metadata      string
}

type randomMetadata struct {
	Seed          int64        `json:"seed"`
	Passes        int          `json:"passes"`
	CenterReal    float64      `json:"center_real"`
	CenterImag    float64      `json:"center_imag"`
	Zoom          float64      `json:"zoom"`
	Palette       string       `json:"palette"`
	Iterations    int          `json:"iterations"`
	MaxIterations int          `json:"max_iterations"`
	Adaptive      bool         `json:"adaptive"`
	Width         int          `json:"width"`
	Height        int          `json:"height"`
	Smooth        bool         `json:"smooth"`
	Steps         []randomStep `json:"steps"`
}

type randomStep struct {
	CenterReal float64 `json:"center_real"`
	CenterImag float64 `json:"center_imag"`
	ZoomFactor float64 `json:"zoom_factor"`
	Complexity int     `json:"complexity"`
}

// Run executes the public command-line interface.
func Run(args []string, stdout, stderr io.Writer, version, executable string) error {
	if len(args) == 1 && args[0] == "--version" {
		_, err := fmt.Fprintln(stdout, version)
		return err
	}
	if len(args) == 2 && args[0] == "palettes" && args[1] == "list" {
		for _, name := range palette.Names() {
			if _, err := fmt.Fprintln(stdout, name); err != nil {
				return err
			}
		}
		return nil
	}
	if len(args) >= 1 && args[0] == "random" {
		options, err := parseRandomOptions(args[1:], stderr)
		if err != nil {
			return err
		}
		return randomRender(options, stdout, stderr, executable)
	}
	if len(args) < 2 || args[0] != "render" || args[1] != "mandelbrot" {
		return fmt.Errorf("usage: %s render mandelbrot [options], %s random [options], %s palettes list, or %s --version", executable, executable, executable, executable)
	}
	options, err := parseRenderOptions(args[2:], stderr)
	if err != nil {
		return err
	}
	return render(options, stdout, stderr)
}

func parseRenderOptions(args []string, stderr io.Writer) (renderOptions, error) {
	o := renderOptions{}
	fs := flag.NewFlagSet("render mandelbrot", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&o.output, "output", "", "PNG output path, or - for stdout")
	fs.StringVar(&o.outputDir, "output-dir", "", "directory for the next fracNNN.png file")
	fs.IntVar(&o.width, "width", defaultWidth, "image width in pixels")
	fs.IntVar(&o.height, "height", defaultHeight, "image height in pixels")
	fs.StringVar(&o.size, "size", "", "image size as WIDTHxHEIGHT")
	fs.StringVar(&o.center, "center", "-0.5,0", "complex center as REAL,IMAG or REAL+IMAGi")
	fs.Float64Var(&o.zoom, "zoom", 1, "linear zoom factor")
	fs.IntVar(&o.iterations, "iterations", defaultIterations, "maximum iterations")
	fs.IntVar(&o.maxIterations, "max-iterations", defaultAdaptiveCap, "adaptive iteration ceiling")
	fs.BoolVar(&o.adaptive, "adaptive", true, "adaptively refine interior boundary iterations")
	fs.StringVar(&o.palette, "palette", "rgb", "color palette")
	fs.BoolVar(&o.smooth, "smooth", false, "use smooth escape coloring")
	fs.IntVar(&o.threads, "threads", runtime.NumCPU(), "render worker count")
	fs.BoolVar(&o.force, "force", false, "overwrite an existing output file")
	fs.BoolVar(&o.unsafe, "unsafe-limits", false, "allow dimensions and iterations over safe limits")
	fs.BoolVar(&o.quiet, "quiet", false, "suppress status messages")
	fs.BoolVar(&o.progress, "progress", false, "report row progress to stderr")
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	if fs.NArg() != 0 {
		return o, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	if o.output == "" && o.outputDir == "" {
		return o, errors.New("--output or --output-dir is required")
	}
	if o.output != "" && o.outputDir != "" {
		return o, errors.New("--output cannot be combined with --output-dir")
	}
	if o.output == "-" && o.force {
		return o, errors.New("--force cannot be used with --output -")
	}
	if o.outputDir != "" && o.force {
		return o, errors.New("--force cannot be used with --output-dir")
	}
	var hasWidth, hasHeight, hasSize bool
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "width":
			hasWidth = true
		case "height":
			hasHeight = true
		case "size":
			hasSize = true
		}
	})
	if hasSize && (hasWidth || hasHeight) {
		return o, errors.New("--size cannot be combined with --width or --height")
	}
	if hasSize {
		width, height, err := parseSize(o.size)
		if err != nil {
			return o, err
		}
		o.width, o.height = width, height
	}
	if _, err := parseCenter(o.center); err != nil {
		return o, err
	}
	if o.zoom <= 0 || !isFinite(o.zoom) {
		return o, errors.New("--zoom must be a finite positive number")
	}
	if o.width <= 0 || o.height <= 0 || o.iterations <= 0 || o.maxIterations <= 0 || o.threads <= 0 {
		return o, errors.New("dimensions, iterations, and threads must be positive")
	}
	if _, found := palette.Lookup(o.palette); !found {
		return o, fmt.Errorf("unknown palette %q", o.palette)
	}
	if !o.unsafe && (o.width > safeSide || o.height > safeSide || int64(o.width)*int64(o.height) > safePixels || max(o.iterations, o.maxIterations) > safeIterations) {
		return o, errors.New("render exceeds safe limits; use --unsafe-limits to allow it")
	}
	return o, nil
}

func parseRandomOptions(args []string, stderr io.Writer) (randomOptions, error) {
	o := randomOptions{}
	fs := flag.NewFlagSet("random", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&o.output, "output", "", "PNG output path, or - for stdout")
	fs.StringVar(&o.outputDir, "output-dir", "", "directory for the next fracNNN.png file")
	fs.IntVar(&o.width, "width", defaultWidth, "image width in pixels")
	fs.IntVar(&o.height, "height", defaultHeight, "image height in pixels")
	fs.StringVar(&o.size, "size", "", "image size as WIDTHxHEIGHT")
	fs.IntVar(&o.iterations, "iterations", defaultIterations, "initial iterations for search and final render")
	fs.IntVar(&o.maxIterations, "max-iterations", defaultAdaptiveCap, "adaptive iteration ceiling for the final image")
	fs.BoolVar(&o.adaptive, "adaptive", true, "adaptively refine the final image")
	fs.StringVar(&o.palette, "palette", "rgb", "color palette")
	fs.BoolVar(&o.randomPalette, "random-palette", false, "select a random color palette")
	fs.BoolVar(&o.smooth, "smooth", true, "use smooth escape coloring")
	fs.IntVar(&o.threads, "threads", runtime.NumCPU(), "render worker count")
	fs.BoolVar(&o.force, "force", false, "overwrite an existing output file")
	fs.BoolVar(&o.unsafe, "unsafe-limits", false, "allow dimensions and iterations over safe limits")
	fs.BoolVar(&o.quiet, "quiet", false, "suppress status messages")
	fs.BoolVar(&o.progress, "progress", false, "report final render progress to stderr")
	fs.Int64Var(&o.seed, "seed", 0, "random seed; generated when omitted")
	fs.IntVar(&o.minPasses, "min-passes", 3, "minimum exploration passes")
	fs.IntVar(&o.maxPasses, "max-passes", 12, "maximum exploration passes")
	fs.Float64Var(&o.minZoom, "min-zoom", 1.5, "minimum per-pass zoom factor")
	fs.Float64Var(&o.maxZoom, "max-zoom", 4, "maximum per-pass zoom factor")
	fs.IntVar(&o.sampleSize, "sample-size", 128, "exploration grid width")
	fs.StringVar(&o.metadata, "metadata", "", "optional JSON metadata output path")
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	if fs.NArg() != 0 {
		return o, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	seedSet := false
	var hasWidth, hasHeight, hasSize bool
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "seed":
			seedSet = true
		case "width":
			hasWidth = true
		case "height":
			hasHeight = true
		case "size":
			hasSize = true
		}
	})
	if !seedSet {
		o.seed = time.Now().UnixNano()
	}
	if o.output == "" && o.outputDir == "" {
		return o, errors.New("--output or --output-dir is required")
	}
	if o.output != "" && o.outputDir != "" {
		return o, errors.New("--output cannot be combined with --output-dir")
	}
	if o.output == "-" && o.force {
		return o, errors.New("--force cannot be used with --output -")
	}
	if o.outputDir != "" && o.force {
		return o, errors.New("--force cannot be used with --output-dir")
	}
	if hasSize && (hasWidth || hasHeight) {
		return o, errors.New("--size cannot be combined with --width or --height")
	}
	if hasSize {
		width, height, err := parseSize(o.size)
		if err != nil {
			return o, err
		}
		o.width, o.height = width, height
	}
	if o.width <= 0 || o.height <= 0 || o.iterations <= 0 || o.maxIterations <= 0 || o.threads <= 0 {
		return o, errors.New("dimensions, iterations, and threads must be positive")
	}
	if o.minPasses < 1 || o.maxPasses < o.minPasses || o.minZoom <= 1 || o.maxZoom < o.minZoom || o.sampleSize < 3 {
		return o, errors.New("passes, zoom factors, and sample size are outside valid ranges")
	}
	if _, found := palette.Lookup(o.palette); !found {
		return o, fmt.Errorf("unknown palette %q", o.palette)
	}
	if !o.unsafe && (o.width > safeSide || o.height > safeSide || int64(o.width)*int64(o.height) > safePixels || max(o.iterations, o.maxIterations) > safeIterations) {
		return o, errors.New("render exceeds safe limits; use --unsafe-limits to allow it")
	}
	return o, nil
}

func render(o renderOptions, stdout, stderr io.Writer) error {
	center, _ := parseCenter(o.center)
	if o.zoom > 1e13 {
		fmt.Fprintln(stderr, "warning: zoom may exceed useful float64 precision")
	}
	if !o.quiet {
		fmt.Fprintf(stderr, "rendering %dx%d, %d iterations, palette %s\n", o.width, o.height, o.iterations, o.palette)
	}
	selected, _ := palette.Lookup(o.palette)
	img := image.NewRGBA(image.Rect(0, 0, o.width, o.height))
	viewWidth := defaultViewportWide / o.zoom
	fractal.Render(img, fractal.RenderOptions{
		Center:     center,
		ViewWidth:  viewWidth,
		ViewHeight: viewWidth * float64(o.height) / float64(o.width),
		Iterations: o.iterations,
		Smooth:     o.smooth,
		Formula:    fractal.Mandelbrot,
		Colorize: func(result fractal.Result) color.RGBA {
			if !result.Escaped {
				return color.RGBA{A: 255}
			}
			return selected(result.Value)
		},
		Threads:  o.threads,
		Progress: o.progress,
		Stderr:   stderr,
		Adaptive: adaptiveOptions(o.adaptive, o.maxIterations, 0),
	})
	output, err := writeOutput(o.output, o.outputDir, o.force, img, stdout)
	if err != nil {
		return err
	}
	if !o.quiet && output != "-" {
		fmt.Fprintln(stderr, "wrote", output)
	}
	return nil
}

func randomRender(o randomOptions, stdout, stderr io.Writer, executable string) error {
	random := rand.New(rand.NewSource(o.seed))
	passes := o.minPasses + random.Intn(o.maxPasses-o.minPasses+1)
	center := complex(-.5, 0)
	viewWidth := defaultViewportWide
	viewHeight := viewWidth * float64(o.height) / float64(o.width)
	steps := make([]randomStep, 0, passes)
	searchIterations := 0
	for pass := 0; pass < passes; pass++ {
		scanIterations := fractal.AndroidProgressiveIterations(defaultViewportWide/viewWidth, o.sampleSize)
		searchIterations = max(searchIterations, scanIterations)
		borders := fractal.FindBorders(fractal.Mandelbrot, center, viewWidth, viewHeight, o.sampleSize, scanIterations)
		if len(borders) == 0 {
			return fmt.Errorf("exploration pass %d found no interior/exterior border", pass+1)
		}
		border := weightedBorder(borders, random)
		factor := o.minZoom + random.Float64()*(o.maxZoom-o.minZoom)
		center += complex(border.RealOffset, border.ImaginaryOffset)
		viewWidth /= factor
		viewHeight /= factor
		steps = append(steps, randomStep{real(center), imag(center), factor, border.Complexity})
	}
	name := o.palette
	if o.randomPalette {
		names := palette.Names()
		name = names[random.Intn(len(names))]
	}
	selected, _ := palette.Lookup(name)
	if !o.quiet {
		fmt.Fprintf(stderr, "explored %d passes with seed %d; rendering palette %s\n", passes, o.seed, name)
	}
	img := image.NewRGBA(image.Rect(0, 0, o.width, o.height))
	fractal.Render(img, fractal.RenderOptions{
		Center: center, ViewWidth: viewWidth, ViewHeight: viewHeight, Iterations: o.iterations,
		Smooth: o.smooth, Formula: fractal.Mandelbrot, Threads: o.threads, Progress: o.progress, Stderr: stderr,
		Adaptive: adaptiveOptions(o.adaptive, o.maxIterations, searchIterations),
		Colorize: func(result fractal.Result) color.RGBA {
			if !result.Escaped {
				return color.RGBA{A: 255}
			}
			return selected(result.Value)
		},
	})
	output, err := writeOutput(o.output, o.outputDir, o.force, img, stdout)
	if err != nil {
		return err
	}
	zoom := defaultViewportWide / viewWidth
	if o.metadata != "" {
		metadata := randomMetadata{
			Seed: o.seed, Passes: passes, CenterReal: real(center), CenterImag: imag(center), Zoom: zoom,
			Palette: name, Iterations: o.iterations, MaxIterations: max(o.maxIterations, searchIterations), Adaptive: o.adaptive,
			Width: o.width, Height: o.height, Smooth: o.smooth, Steps: steps,
		}
		if err := writeMetadata(o.metadata, o.force, metadata); err != nil {
			return err
		}
	}
	fmt.Fprintln(stderr, reproductionCommand(center, zoom, o, max(o.maxIterations, searchIterations), name, executable))
	if !o.quiet && output != "-" {
		fmt.Fprintln(stderr, "wrote", output)
	}
	return nil
}

func reproductionCommand(center complex128, zoom float64, options randomOptions, maxIterations int, paletteName, executable string) string {
	centerValue := fmt.Sprintf("%.17g,%.17g", real(center), imag(center))
	return fmt.Sprintf(
		"reproduce with: %s render mandelbrot --center %q --zoom %.17g --iterations %d --max-iterations %d --adaptive=%t --palette %s --smooth=%t --width %d --height %d --threads %d --output reproduced.png",
		executable, centerValue, zoom, options.iterations, maxIterations, options.adaptive, paletteName, options.smooth, options.width, options.height, options.threads,
	)
}

func weightedBorder(borders []fractal.Border, random *rand.Rand) fractal.Border {
	total := 0
	for _, border := range borders {
		total += border.Complexity
	}
	pick := random.Intn(total)
	for _, border := range borders {
		pick -= border.Complexity
		if pick < 0 {
			return border
		}
	}
	return borders[len(borders)-1]
}

func adaptiveOptions(enabled bool, maxIterations, minimumIterations int) *fractal.AdaptiveOptions {
	if !enabled {
		return nil
	}
	return &fractal.AdaptiveOptions{MaxIterations: maxIterations, MaxRounds: defaultAdaptiveRuns, MinimumIterations: minimumIterations}
}

func writeOutput(output, outputDir string, force bool, img image.Image, stdout io.Writer) (string, error) {
	if outputDir != "" {
		return writeSequentialPNG(outputDir, img)
	}
	if output == "-" {
		return output, png.Encode(stdout, img)
	}
	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if !force {
		flags = os.O_WRONLY | os.O_CREATE | os.O_EXCL
	}
	file, err := os.OpenFile(output, flags, 0o644)
	if err != nil {
		return "", err
	}
	err = png.Encode(file, img)
	closeErr := file.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	return output, nil
}

func writeSequentialPNG(directory string, img image.Image) (string, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return "", err
	}
	number := -1
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, "frac") || !strings.HasSuffix(name, ".png") {
			continue
		}
		value, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(name, "frac"), ".png"))
		if err == nil && value > number {
			number = value
		}
	}
	for number++; ; number++ {
		output := filepath.Join(directory, fmt.Sprintf("frac%03d.png", number))
		file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		err = png.Encode(file, img)
		closeErr := file.Close()
		if err != nil {
			return "", err
		}
		if closeErr != nil {
			return "", closeErr
		}
		return output, nil
	}
}

func writeMetadata(output string, force bool, metadata randomMetadata) error {
	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if !force {
		flags = os.O_WRONLY | os.O_CREATE | os.O_EXCL
	}
	file, err := os.OpenFile(output, flags, 0o644)
	if err != nil {
		return err
	}
	err = json.NewEncoder(file).Encode(metadata)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func parseSize(value string) (int, int, error) {
	parts := strings.Split(strings.ToLower(value), "x")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid --size %q; expected WIDTHxHEIGHT", value)
	}
	width, widthErr := strconv.Atoi(parts[0])
	height, heightErr := strconv.Atoi(parts[1])
	if widthErr != nil || heightErr != nil {
		return 0, 0, fmt.Errorf("invalid --size %q", value)
	}
	return width, height, nil
}

func parseCenter(value string) (complex128, error) {
	compact := strings.ReplaceAll(value, " ", "")
	if parts := strings.Split(compact, ","); len(parts) == 2 {
		realPart, realErr := strconv.ParseFloat(parts[0], 64)
		imaginaryPart, imaginaryErr := strconv.ParseFloat(parts[1], 64)
		if realErr == nil && imaginaryErr == nil && isFinite(realPart) && isFinite(imaginaryPart) {
			return complex(realPart, imaginaryPart), nil
		}
	}
	parsed, err := strconv.ParseComplex(compact, 128)
	if err != nil || !isFinite(real(parsed)) || !isFinite(imag(parsed)) {
		return 0, fmt.Errorf("invalid --center %q; expected REAL,IMAG or REAL+IMAGi", value)
	}
	return parsed, nil
}

func isFinite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
