package main

import (
	"github.com/archstats/archstats/cmd"
	"os"
)

func main() {
	err := cmd.Execute(os.Stdout, os.Stderr, nil, os.Args[1:])
	if err != nil {
		os.Exit(1)
	}
}
