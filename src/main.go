package main

import (
	"os"

	"github.com/mieubrisse/yappblocker/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
