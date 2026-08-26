package output

import (
	"bytes"
	"io"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

const absentProposedCaptureText = "(none)"

// WriteProposedCaptureSummary writes a terminal-safe confirmation summary.
// Validation completes before any output is written.
func WriteProposedCaptureSummary(w io.Writer, capture domain.ProposedCapture) error {
	if err := capture.Validate(); err != nil {
		return err
	}

	var rendered bytes.Buffer
	rendered.WriteString("Capture summary\n")
	writeSummaryField(&rendered, "Type", capture.Type.String())
	writeSummaryField(&rendered, "Description", EscapeText(capture.Description))
	if capture.Type == domain.CaptureTypeFriction {
		details := capture.Details.Friction
		writeSummaryField(&rendered, "Project", escapedProposedCaptureText(details.Project))
		writeSummaryField(&rendered, "Frequency", details.Frequency.String())
		writeSummaryField(&rendered, "Impact", details.Impact.String())
		writeSummaryField(&rendered, "Category", details.Category.String())
		writeSummaryField(&rendered, "Current workaround", escapedProposedCaptureText(details.CurrentWorkaround))
	}
	_, err := io.Copy(w, &rendered)
	return err
}

func writeSummaryField(w *bytes.Buffer, label, value string) {
	w.WriteString(label)
	w.WriteString(": ")
	w.WriteString(value)
	w.WriteByte('\n')
}

func escapedProposedCaptureText(value *string) string {
	if value == nil {
		return absentProposedCaptureText
	}
	return EscapeText(*value)
}
