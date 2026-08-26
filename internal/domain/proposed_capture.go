package domain

// ProposedCapture is a fully collected and normalized capture before persistence
// metadata is generated.
type ProposedCapture struct {
	Type        CaptureType
	Description string
	Details     ProposedCaptureDetails
}

// ProposedCaptureDetails holds exactly one details shape matching the capture
// type. Empty detail types remain explicit so future workflows can evolve
// independently.
type ProposedCaptureDetails struct {
	Friction *FrictionCaptureDetails
	Action   *ActionCaptureDetails
	FollowUp *FollowUpCaptureDetails
	Decision *DecisionCaptureDetails
}

// FrictionCaptureDetails contains the current friction-specific fields.
type FrictionCaptureDetails struct {
	Project           *string
	Frequency         Frequency
	Impact            Impact
	Category          Category
	CurrentWorkaround *string
}

// ActionCaptureDetails is the initial, deliberately minimal action shape.
type ActionCaptureDetails struct{}

// FollowUpCaptureDetails is the initial, deliberately minimal follow-up shape.
type FollowUpCaptureDetails struct{}

// DecisionCaptureDetails is the initial, deliberately minimal decision shape.
type DecisionCaptureDetails struct{}

// ProposedCaptureInput contains collected values before domain normalization.
type ProposedCaptureInput struct {
	Type        CaptureType
	Description string
	Details     ProposedCaptureDetailsInput
}

// ProposedCaptureDetailsInput holds exactly one collected details shape.
type ProposedCaptureDetailsInput struct {
	Friction *FrictionCaptureInput
	Action   *ActionCaptureDetails
	FollowUp *FollowUpCaptureDetails
	Decision *DecisionCaptureDetails
}

// FrictionCaptureInput contains collected friction values before normalization.
type FrictionCaptureInput struct {
	Project           string
	Frequency         Frequency
	Impact            Impact
	Category          Category
	CurrentWorkaround string
}

// NewProposedCapture constructs a canonical proposed capture.
func NewProposedCapture(input ProposedCaptureInput) (ProposedCapture, error) {
	if !input.Type.Valid() {
		return ProposedCapture{}, &InvalidValueError{Field: "capture type", Value: input.Type.String()}
	}
	description, err := NormalizeDescription(input.Description)
	if err != nil {
		return ProposedCapture{}, err
	}
	if !input.Details.matches(input.Type) {
		return ProposedCapture{}, &InvalidValueError{Field: "details", Value: input.Type.String()}
	}

	proposed := ProposedCapture{
		Type:        input.Type,
		Description: description,
	}
	switch input.Type {
	case CaptureTypeFriction:
		friction, err := newFrictionCaptureDetails(*input.Details.Friction)
		if err != nil {
			return ProposedCapture{}, err
		}
		proposed.Details.Friction = &friction
	case CaptureTypeAction:
		proposed.Details.Action = &ActionCaptureDetails{}
	case CaptureTypeFollowUp:
		proposed.Details.FollowUp = &FollowUpCaptureDetails{}
	case CaptureTypeDecision:
		proposed.Details.Decision = &DecisionCaptureDetails{}
	}
	return proposed, nil
}

// Validate checks that a proposed capture is canonical and internally matched.
func (p ProposedCapture) Validate() error {
	if !p.Type.Valid() {
		return &InvalidValueError{Field: "capture type", Value: p.Type.String()}
	}
	description, err := NormalizeDescription(p.Description)
	if err != nil {
		return err
	}
	if description != p.Description {
		return &InvalidValueError{Field: "description", Value: p.Description}
	}
	if !p.Details.matches(p.Type) {
		return &InvalidValueError{Field: "details", Value: p.Type.String()}
	}
	if p.Type == CaptureTypeFriction {
		return p.Details.Friction.validate()
	}
	return nil
}

func (d ProposedCaptureDetailsInput) matches(captureType CaptureType) bool {
	return captureDetailsMatch(
		captureType,
		d.Friction != nil,
		d.Action != nil,
		d.FollowUp != nil,
		d.Decision != nil,
	)
}

func (d ProposedCaptureDetails) matches(captureType CaptureType) bool {
	return captureDetailsMatch(
		captureType,
		d.Friction != nil,
		d.Action != nil,
		d.FollowUp != nil,
		d.Decision != nil,
	)
}

func captureDetailsMatch(captureType CaptureType, friction, action, followUp, decision bool) bool {
	count := 0
	if friction {
		count++
	}
	if action {
		count++
	}
	if followUp {
		count++
	}
	if decision {
		count++
	}
	if count != 1 {
		return false
	}
	switch captureType {
	case CaptureTypeFriction:
		return friction
	case CaptureTypeAction:
		return action
	case CaptureTypeFollowUp:
		return followUp
	case CaptureTypeDecision:
		return decision
	default:
		return false
	}
}

func newFrictionCaptureDetails(input FrictionCaptureInput) (FrictionCaptureDetails, error) {
	details := FrictionCaptureDetails{
		Project:           NormalizeOptionalText(input.Project),
		Frequency:         input.Frequency,
		Impact:            input.Impact,
		Category:          input.Category,
		CurrentWorkaround: NormalizeOptionalText(input.CurrentWorkaround),
	}
	if err := details.validate(); err != nil {
		return FrictionCaptureDetails{}, err
	}
	return details, nil
}

func (d FrictionCaptureDetails) validate() error {
	if !optionalTextIsNormalized(d.Project) {
		return &InvalidValueError{Field: "project", Value: *d.Project}
	}
	if !d.Frequency.Valid() {
		return &InvalidValueError{Field: "frequency", Value: d.Frequency.String()}
	}
	if !d.Impact.Valid() {
		return &InvalidValueError{Field: "impact", Value: d.Impact.String()}
	}
	if !d.Category.Valid() {
		return &InvalidValueError{Field: "category", Value: d.Category.String()}
	}
	if !optionalTextIsNormalized(d.CurrentWorkaround) {
		return &InvalidValueError{Field: "current workaround", Value: *d.CurrentWorkaround}
	}
	return nil
}
