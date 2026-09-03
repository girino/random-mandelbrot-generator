package main

import (
	"fmt"
	"os"

	"github.com/girino/random-mandelbrot-generator/internal/cli"
)

var version = "v0.1.0"

func main() {
	if err := cli.Run(os.Args[1:], os.Stdout, os.Stderr, version); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
