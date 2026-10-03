package main

import (
	"flag"
	"fmt"
	"os"

	"example.com/product-variant-catalog/internal/cli"
)

func main() {
	stateDir := flag.String("state", "", "directory holding catalog state")
	flag.Parse()

	if *stateDir == "" {
		fmt.Fprintln(os.Stderr, "usage: catalog --state <state-dir>")
		os.Exit(2)
	}

	engine, err := cli.NewEngine(*stateDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot load state: %v\n", err)
		os.Exit(1)
	}
	if err := engine.Run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}
