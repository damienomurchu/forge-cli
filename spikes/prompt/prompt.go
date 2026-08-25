// Package promptprobe contains the disposable Phase 1 interactive prompt spike.
package promptprobe

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/charmbracelet/huh"
)

var (
	ErrCancelled = errors.New("prompt cancelled")
	ErrEOF       = errors.New("prompt input closed")
)

type Prompt struct {
	Input  io.Reader
	Output io.Writer
}

func (p Prompt) Select(ctx context.Context, label string, choices []string, defaultValue string) (string, error) {
	if len(choices) == 0 {
		return "", errors.New("select requires at least one choice")
	}
	validDefault := false
	options := make([]huh.Option[string], len(choices))
	for i, choice := range choices {
		if choice == defaultValue {
			validDefault = true
		}
		options[i] = huh.NewOption(choice, choice)
	}
	if !validDefault {
		return "", fmt.Errorf("default value %q is not a choice", defaultValue)
	}

	value := defaultValue
	field := huh.NewSelect[string]().
		Title(label).
		Options(options...).
		Value(&value)
	if err := p.run(ctx, field); err != nil {
		return "", err
	}
	return value, nil
}

func (p Prompt) Text(ctx context.Context, label string) (string, error) {
	var value string
	field := huh.NewInput().Title(label).Value(&value)
	if err := p.run(ctx, field); err != nil {
		return "", err
	}
	return value, nil
}

func (p Prompt) Confirm(ctx context.Context, label string, defaultValue bool) (bool, error) {
	value := defaultValue
	field := huh.NewConfirm().Title(label).Value(&value)
	if err := p.run(ctx, field); err != nil {
		return false, err
	}
	return value, nil
}

func (p Prompt) run(ctx context.Context, field huh.Field) error {
	if p.Input == nil || p.Output == nil {
		return errors.New("prompt input and output are required")
	}
	runCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	input := &eofReader{Reader: p.Input, onImmediateEOF: func() { cancel(ErrEOF) }}
	form := huh.NewForm(huh.NewGroup(field)).
		WithInput(input).
		WithOutput(p.Output).
		WithShowHelp(false)
	err := form.RunWithContext(runCtx)
	if err == nil {
		return nil
	}
	if errors.Is(context.Cause(runCtx), ErrEOF) {
		return ErrEOF
	}
	if errors.Is(err, huh.ErrUserAborted) {
		return ErrCancelled
	}
	if errors.Is(err, io.EOF) {
		return ErrEOF
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return fmt.Errorf("run prompt: %w", err)
}

type eofReader struct {
	io.Reader
	sawInput       bool
	onImmediateEOF func()
}

func (r *eofReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if n > 0 {
		r.sawInput = true
	}
	if errors.Is(err, io.EOF) && !r.sawInput {
		r.onImmediateEOF()
	}
	return n, err
}
