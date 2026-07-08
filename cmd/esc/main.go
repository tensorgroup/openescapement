package main

import (
	"fmt"
	"os"

	"github.com/tensorgroup/openescapement/internal/cli"
)

func main() {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "esc: %v\n", err)
		os.Exit(4)
	}
	os.Exit(cli.Run(root, os.Args[1:], os.Stdout, os.Stderr))
}
