package main

import (
	"github.com/RyanSusana/archstats/cmd"
	"os"
)

func main() {
	err := cmd.Execute(os.Stdout, os.Stderr, os.Args[1:])
	if err != nil {
		os.Exit(1)
	}
}
