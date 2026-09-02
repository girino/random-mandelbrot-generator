package cli

import (
	"bytes"
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseCenter(t *testing.T) {
	for _, test := range []struct {
		input string
		want  complex128
	}{
		{"-0.5,0", complex(-0.5, 0)},
		{"-0.25+.37i", complex(-0.25, .37)},
		{"-0.25 + .37 i", complex(-0.25, .37)},
	} {
		got, err := parseCenter(test.input)
		if err != nil || got != test.want {
			t.Errorf("parseCenter(%q) = %v, %v; want %v, nil", test.input, got, err, test.want)
		}
	}
}

func TestRenderValidation(t *testing.T) {
	for _, args := range [][]string{
		{"--output", "out.png", "--size", "3x3", "--width", "3"},
		{"--output", "out.png", "--palette", "unknown"},
		{"--output", "out.png", "--width", "16385"},
	} {
		if _, err := parseRenderOptions(args, &bytes.Buffer{}); err == nil {
			t.Errorf("parseRenderOptions(%v) succeeded; want error", args)
		}
	}
}

func TestAdaptiveDefaults(t *testing.T) {
	options, err := parseRenderOptions([]string{"--output", "out.png"}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if !options.adaptive || options.iterations != 64 || options.maxIterations != 262144 {
		t.Fatalf("unexpected adaptive defaults: %#v", options)
	}
}

func TestRunWritesPNGToStandardOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run([]string{"render", "mandelbrot", "--size", "2x2", "--iterations", "4", "--quiet", "--output", "-"}, &stdout, &stderr, "v0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q; want no status for --quiet", stderr.String())
	}
	if _, err := png.Decode(bytes.NewReader(stdout.Bytes())); err != nil {
		t.Fatalf("stdout is not a PNG: %v", err)
	}
}

func TestVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"--version"}, &stdout, &stderr, "v0.1.0"); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "v0.1.0\n" || stderr.Len() != 0 {
		t.Fatalf("version output = stdout %q, stderr %q", stdout.String(), stderr.String())
	}
}

func TestRandomRenderIsReproducible(t *testing.T) {
	args := []string{"random", "--output", "-", "--quiet", "--seed", "7", "--min-passes", "1", "--max-passes", "1", "--sample-size", "16", "--iterations", "64", "--size", "8x8", "--palette", "fire"}
	var first, firstErr bytes.Buffer
	if err := Run(args, &first, &firstErr, "v0.1.0"); err != nil {
		t.Fatal(err)
	}
	var second, secondErr bytes.Buffer
	if err := Run(args, &second, &secondErr, "v0.1.0"); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("fixed random seed produced different PNG output")
	}
	if _, err := png.Decode(bytes.NewReader(first.Bytes())); err != nil {
		t.Fatalf("random stdout is not a PNG: %v", err)
	}
}

func TestWriteMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metadata.json")
	want := randomMetadata{Seed: 7, Passes: 1, Zoom: 2, Palette: "fire", Width: 8, Height: 8, Smooth: true}
	if err := writeMetadata(path, false, want); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var got randomMetadata
	if err := json.NewDecoder(file).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("metadata = %#v; want %#v", got, want)
	}
}
