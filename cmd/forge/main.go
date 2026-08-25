package main

import (
	"context"
	"crypto/rand"
	"os"
	"runtime"
	"time"

	"github.com/damienomurchu/forge-cli/internal/cli"
)

var version = "dev"

func main() {
	rt := cli.Runtime{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Env:    os.Getenv,
		Now:    time.Now,
		Random: rand.Reader,
		IsTTY:  stdinIsTerminal,
		GOOS:   runtime.GOOS,
		EUID:   os.Geteuid(),
	}

	if err := cli.Run(context.Background(), os.Args[1:], rt, version); err != nil {
		os.Exit(cli.WriteError(os.Stderr, err))
	}
}
