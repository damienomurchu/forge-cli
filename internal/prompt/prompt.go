// Package prompt adapts Forge's interactive prompt primitives to terminal UI.
package prompt

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

// Adapter provides interactive prompts over injected streams.
type Adapter struct {
	input  io.Reader
	output io.Writer
}

// New constructs a prompt adapter without initializing terminal UI.
func New(input io.Reader, output io.Writer) *Adapter {
	return &Adapter{input: input, output: output}
}

func (p *Adapter) Select(ctx context.Context, label string, choices []string, defaultValue string) (string, error) {
	if len(choices) == 0 {
		return "", errors.New("select requires at least one choice")
	}
	validDefault := false
	options := make([]huh.Option[string], len(choices))
	for index, choice := range choices {
		if choice == defaultValue {
			validDefault = true
		}
		options[index] = huh.NewOption(choice, choice)
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

func (p *Adapter) Text(ctx context.Context, label string) (string, error) {
	var value string
	field := huh.NewInput().Title(label).Value(&value)
	if err := p.run(ctx, field); err != nil {
		return "", err
	}
	return value, nil
}

func (p *Adapter) Confirm(ctx context.Context, label string, defaultValue bool) (bool, error) {
	value := defaultValue
	field := huh.NewConfirm().Title(label).Value(&value)
	if err := p.run(ctx, field); err != nil {
		return false, err
	}
	return value, nil
}

func (p *Adapter) run(ctx context.Context, field huh.Field) error {
	if p == nil || p.input == nil || p.output == nil {
		return errors.New("prompt input and output are required")
	}
	runContext, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	input := &eofReader{Reader: p.input, onImmediateEOF: func() { cancel(ErrEOF) }}
	form := huh.NewForm(huh.NewGroup(field)).
		WithInput(input).
		WithOutput(p.output).
		WithShowHelp(false)
	err := form.RunWithContext(runContext)
	if err == nil {
		return nil
	}
	if errors.Is(context.Cause(runContext), ErrEOF) {
		return ErrEOF
	}
	if errors.Is(err, huh.ErrUserAborted) {
		return ErrCancelled
	}
	if errors.Is(err, io.EOF) {
		return ErrEOF
	}
	if contextError := ctx.Err(); contextError != nil {
		return contextError
	}
	return fmt.Errorf("run prompt: %w", err)
}

type eofReader struct {
	io.Reader
	sawInput       bool
	onImmediateEOF func()
}

func (r *eofReader) Read(buffer []byte) (int, error) {
	count, err := r.Reader.Read(buffer)
	if count > 0 {
		r.sawInput = true
	}
	if errors.Is(err, io.EOF) && !r.sawInput {
		r.onImmediateEOF()
	}
	return count, err
}
