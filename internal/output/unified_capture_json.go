package output

import (
	"encoding/json"
	"io"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

// WriteCaptureJSON writes one validated unified capture as a JSON object
// followed by a newline.
func WriteCaptureJSON(w io.Writer, capture domain.Capture) error {
	if err := capture.Validate(); err != nil {
		return err
	}
	return json.NewEncoder(w).Encode(captureToJSON(capture))
}

// WriteCapturesJSON writes validated unified captures as one JSON array followed
// by a newline. Every capture is validated before any output is written.
func WriteCapturesJSON(w io.Writer, captures []domain.Capture) error {
	encoded := make([]unifiedCaptureJSON, len(captures))
	for i, capture := range captures {
		if err := capture.Validate(); err != nil {
			return err
		}
		encoded[i] = captureToJSON(capture)
	}
	return json.NewEncoder(w).Encode(encoded)
}

func captureToJSON(capture domain.Capture) unifiedCaptureJSON {
	var details any
	if capture.Type == domain.CaptureTypeFriction {
		friction := capture.Details.Friction
		details = unifiedFrictionDetailsJSON{
			Project:           friction.Project,
			Frequency:         friction.Frequency.String(),
			Impact:            friction.Impact.String(),
			Category:          friction.Category.String(),
			CurrentWorkaround: friction.CurrentWorkaround,
		}
	} else {
		details = emptyCaptureDetailsJSON{}
	}
	return unifiedCaptureJSON{
		ID:          capture.ID.String(),
		CaptureType: capture.Type.String(),
		Description: capture.Description,
		Details:     details,
		CreatedAt:   capture.CreatedAt.String(),
		UpdatedAt:   capture.UpdatedAt.String(),
	}
}

type unifiedCaptureJSON struct {
	ID          string `json:"id"`
	CaptureType string `json:"capture_type"`
	Description string `json:"description"`
	Details     any    `json:"details"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type unifiedFrictionDetailsJSON struct {
	Project           *string `json:"project"`
	Frequency         string  `json:"frequency"`
	Impact            string  `json:"impact"`
	Category          string  `json:"category"`
	CurrentWorkaround *string `json:"current_workaround"`
}

type emptyCaptureDetailsJSON struct{}
