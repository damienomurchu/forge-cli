package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/damienomurchu/forge-cli/spikes/prompt"
)

func main() {
	mode := flag.String("mode", "noop", "noop, select, confirm, or text")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: prompt-spike [-mode MODE]")
		os.Exit(2)
	}
	if *mode == "noop" {
		return
	}

	prompt := promptprobe.Prompt{Input: os.Stdin, Output: os.Stderr}
	var err error
	switch *mode {
	case "select":
		_, err = prompt.Select(context.Background(), "Kind", []string{"thought", "idea"}, "thought")
	case "confirm":
		_, err = prompt.Confirm(context.Background(), "Save?", true)
	case "text":
		_, err = prompt.Text(context.Background(), "Workaround")
	default:
		fmt.Fprintf(os.Stderr, "unknown mode: %s\n", *mode)
		os.Exit(2)
	}
	if errors.Is(err, promptprobe.ErrCancelled) {
		fmt.Fprintln(os.Stderr, "cancelled")
		os.Exit(130)
	}
	if errors.Is(err, promptprobe.ErrEOF) {
		fmt.Fprintln(os.Stderr, "input closed")
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
