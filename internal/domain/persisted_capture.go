package domain

import (
	"io"
	"time"
)

// Capture is Forge's unified persisted domain record.
type Capture struct {
	ID          ID
	Type        CaptureType
	Description string
	Details     ProposedCaptureDetails
	CreatedAt   Timestamp
	UpdatedAt   Timestamp
}

// NewPersistedCapture adds persistence metadata to a validated proposal. The
// proposal is validated before randomness is consumed.
func NewPersistedCapture(proposed ProposedCapture, now time.Time, random io.Reader) (Capture, error) {
	if err := proposed.Validate(); err != nil {
		return Capture{}, err
	}
	id, err := GenerateCaptureID(random)
	if err != nil {
		return Capture{}, err
	}
	timestamp := NewTimestamp(now)
	return Capture{
		ID:          id,
		Type:        proposed.Type,
		Description: proposed.Description,
		Details:     cloneProposedCaptureDetails(proposed.Details),
		CreatedAt:   timestamp,
		UpdatedAt:   timestamp,
	}, nil
}

// Validate checks that a persisted capture is canonical and internally matched.
func (c Capture) Validate() error {
	if !c.Type.Valid() {
		return &InvalidValueError{Field: "capture type", Value: c.Type.String()}
	}
	if err := ValidateCaptureID(c.ID, c.Type); err != nil {
		return err
	}
	description, err := NormalizeDescription(c.Description)
	if err != nil {
		return err
	}
	if description != c.Description {
		return &InvalidValueError{Field: "description", Value: c.Description}
	}
	if !c.Details.matches(c.Type) {
		return &InvalidValueError{Field: "details", Value: c.Type.String()}
	}
	if c.Type == CaptureTypeFriction {
		if err := c.Details.Friction.validate(); err != nil {
			return err
		}
	}
	if c.UpdatedAt.Time().Before(c.CreatedAt.Time()) {
		return &InvalidValueError{Field: "updated_at", Value: c.UpdatedAt.String()}
	}
	return nil
}

func cloneProposedCaptureDetails(details ProposedCaptureDetails) ProposedCaptureDetails {
	cloned := ProposedCaptureDetails{}
	if details.Friction != nil {
		friction := *details.Friction
		friction.Project = cloneOptionalText(friction.Project)
		friction.CurrentWorkaround = cloneOptionalText(friction.CurrentWorkaround)
		cloned.Friction = &friction
	}
	if details.Action != nil {
		cloned.Action = &ActionCaptureDetails{}
	}
	if details.FollowUp != nil {
		cloned.FollowUp = &FollowUpCaptureDetails{}
	}
	if details.Decision != nil {
		cloned.Decision = &DecisionCaptureDetails{}
	}
	return cloned
}

func cloneOptionalText(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
