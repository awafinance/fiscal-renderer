package main

import (
	"os"

	"github.com/awafinance/fiscal-renderer/internal/bfrepcli"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	os.Exit(bfrepcli.Run(os.Args[1:], os.Stdout, os.Stderr, cwd))
}
