package output

import (
	"bytes"
	"io"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

// WriteCaptureCreated writes the concise confirmation for a newly persisted
// unified capture.
func WriteCaptureCreated(w io.Writer, capture domain.Capture) error {
	if err := capture.Validate(); err != nil {
		return err
	}
	_, err := io.WriteString(
		w,
		"Created "+capture.Type.String()+" capture "+capture.ID.String()+"\n",
	)
	return err
}

// WriteCapture writes a complete terminal-safe human representation of one
// unified capture. Validation completes before any output is written.
func WriteCapture(w io.Writer, capture domain.Capture) error {
	if err := capture.Validate(); err != nil {
		return err
	}

	var rendered bytes.Buffer
	writeCaptureField(&rendered, "ID", capture.ID.String())
	writeCaptureField(&rendered, "Type", capture.Type.String())
	writeCaptureField(&rendered, "Description", EscapeText(capture.Description))
	if capture.Type == domain.CaptureTypeFriction {
		details := capture.Details.Friction
		writeCaptureField(&rendered, "Project", escapedProposedCaptureText(details.Project))
		writeCaptureField(&rendered, "Frequency", details.Frequency.String())
		writeCaptureField(&rendered, "Impact", details.Impact.String())
		writeCaptureField(&rendered, "Category", details.Category.String())
		writeCaptureField(&rendered, "Current workaround", escapedProposedCaptureText(details.CurrentWorkaround))
	}
	writeCaptureField(&rendered, "Created", capture.CreatedAt.String())
	writeCaptureField(&rendered, "Updated", capture.UpdatedAt.String())
	_, err := io.Copy(w, &rendered)
	return err
}

// WriteCaptureList writes one compact terminal-safe line for each unified
// capture in the provided order. Every capture is validated before output.
func WriteCaptureList(w io.Writer, captures []domain.Capture) error {
	for _, capture := range captures {
		if err := capture.Validate(); err != nil {
			return err
		}
	}

	var rendered bytes.Buffer
	for _, capture := range captures {
		rendered.WriteString(capture.ID.String())
		rendered.WriteString("  ")
		rendered.WriteString(capture.Type.String())
		rendered.WriteString("  ")
		rendered.WriteString(EscapeText(capture.Description))
		rendered.WriteByte('\n')
	}
	_, err := io.Copy(w, &rendered)
	return err
}

func writeCaptureField(w *bytes.Buffer, label, value string) {
	w.WriteString(label)
	w.WriteString(": ")
	w.WriteString(value)
	w.WriteByte('\n')
}
