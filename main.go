package main

import (
	"fmt"
	"os"

	"github.com/chand1012/jeb/pkg/commands"
)

func main() {
	if err := commands.NewRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
