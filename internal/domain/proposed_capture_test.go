package domain

import (
	"errors"
	"reflect"
	"testing"
)

func TestNewProposedCaptureConstructsEachType(t *testing.T) {
	tests := []struct {
		name            string
		input           ProposedCaptureInput
		wantDescription string
		assert          func(*testing.T, ProposedCapture)
	}{
		{
			name: "friction",
			input: ProposedCaptureInput{
				Type:        CaptureTypeFriction,
				Description: "  Releases require\nmanual checks  ",
				Details: ProposedCaptureDetailsInput{Friction: &FrictionCaptureInput{
					Project:           "  forge  ",
					Frequency:         FrequencyWeekly,
					Impact:            ImpactHigh,
					Category:          CategoryVerification,
					CurrentWorkaround: "  Follow a checklist  ",
				}},
			},
			wantDescription: "Releases require\nmanual checks",
			assert: func(t *testing.T, got ProposedCapture) {
				t.Helper()
				details := got.Details.Friction
				if details == nil || details.Project == nil || *details.Project != "forge" {
					t.Fatalf("friction project = %v, want forge", details)
				}
				if details.CurrentWorkaround == nil || *details.CurrentWorkaround != "Follow a checklist" {
					t.Errorf("current workaround = %v", details.CurrentWorkaround)
				}
			},
		},
		{
			name: "action",
			input: ProposedCaptureInput{
				Type: CaptureTypeAction, Description: "Take an action",
				Details: ProposedCaptureDetailsInput{Action: &ActionCaptureDetails{}},
			},
			wantDescription: "Take an action",
			assert: func(t *testing.T, got ProposedCapture) {
				t.Helper()
				if got.Details.Action == nil {
					t.Fatal("action details are nil")
				}
			},
		},
		{
			name: "follow-up",
			input: ProposedCaptureInput{
				Type: CaptureTypeFollowUp, Description: "Chase a response",
				Details: ProposedCaptureDetailsInput{FollowUp: &FollowUpCaptureDetails{}},
			},
			wantDescription: "Chase a response",
			assert: func(t *testing.T, got ProposedCapture) {
				t.Helper()
				if got.Details.FollowUp == nil {
					t.Fatal("follow-up details are nil")
				}
			},
		},
		{
			name: "decision",
			input: ProposedCaptureInput{
				Type: CaptureTypeDecision, Description: "Choose an approach",
				Details: ProposedCaptureDetailsInput{Decision: &DecisionCaptureDetails{}},
			},
			wantDescription: "Choose an approach",
			assert: func(t *testing.T, got ProposedCapture) {
				t.Helper()
				if got.Details.Decision == nil {
					t.Fatal("decision details are nil")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewProposedCapture(tt.input)
			if err != nil {
				t.Fatalf("NewProposedCapture() error = %v", err)
			}
			if got.Description != tt.wantDescription {
				t.Errorf("description = %q, want %q", got.Description, tt.wantDescription)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("proposed capture is invalid: %v", err)
			}
			tt.assert(t, got)
		})
	}
}

func TestNewProposedCaptureKeepsOptionalFrictionTextAbsent(t *testing.T) {
	got, err := NewProposedCapture(validProposedFrictionInput())
	if err != nil {
		t.Fatalf("NewProposedCapture() error = %v", err)
	}
	if got.Details.Friction.Project != nil || got.Details.Friction.CurrentWorkaround != nil {
		t.Errorf("optional friction text = %v/%v, want nil/nil",
			got.Details.Friction.Project, got.Details.Friction.CurrentWorkaround)
	}
}

func TestNewProposedCaptureRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*ProposedCaptureInput)
		wantField string
	}{
		{name: "capture type", mutate: func(input *ProposedCaptureInput) { input.Type = "invalid" }, wantField: "capture type"},
		{name: "description", mutate: func(input *ProposedCaptureInput) { input.Description = " \t " }, wantField: "description"},
		{name: "missing details", mutate: func(input *ProposedCaptureInput) { input.Details = ProposedCaptureDetailsInput{} }, wantField: "details"},
		{name: "mismatched details", mutate: func(input *ProposedCaptureInput) {
			input.Details = ProposedCaptureDetailsInput{Action: &ActionCaptureDetails{}}
		}, wantField: "details"},
		{name: "multiple details", mutate: func(input *ProposedCaptureInput) {
			input.Details.Action = &ActionCaptureDetails{}
		}, wantField: "details"},
		{name: "frequency", mutate: func(input *ProposedCaptureInput) { input.Details.Friction.Frequency = "invalid" }, wantField: "frequency"},
		{name: "impact", mutate: func(input *ProposedCaptureInput) { input.Details.Friction.Impact = "invalid" }, wantField: "impact"},
		{name: "category", mutate: func(input *ProposedCaptureInput) { input.Details.Friction.Category = "invalid" }, wantField: "category"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validProposedFrictionInput()
			tt.mutate(&input)
			got, err := NewProposedCapture(input)
			var invalid *InvalidValueError
			if !errors.As(err, &invalid) || invalid.Field != tt.wantField {
				t.Fatalf("error = %T %v, want invalid %s", err, err, tt.wantField)
			}
			if !reflect.DeepEqual(got, ProposedCapture{}) {
				t.Errorf("NewProposedCapture() = %#v, want zero value", got)
			}
		})
	}
}

func TestProposedCaptureValidateRejectsNonCanonicalOrMismatchedValues(t *testing.T) {
	valid, err := NewProposedCapture(validProposedFrictionInput())
	if err != nil {
		t.Fatalf("NewProposedCapture() error = %v", err)
	}
	tests := []struct {
		name      string
		mutate    func(*ProposedCapture)
		wantField string
	}{
		{name: "description", mutate: func(capture *ProposedCapture) { capture.Description = " padded " }, wantField: "description"},
		{name: "mismatched details", mutate: func(capture *ProposedCapture) {
			capture.Details.Friction = nil
			capture.Details.Action = &ActionCaptureDetails{}
		}, wantField: "details"},
		{name: "multiple details", mutate: func(capture *ProposedCapture) { capture.Details.Action = &ActionCaptureDetails{} }, wantField: "details"},
		{name: "project", mutate: func(capture *ProposedCapture) {
			value := " padded "
			capture.Details.Friction.Project = &value
		}, wantField: "project"},
		{name: "workaround", mutate: func(capture *ProposedCapture) {
			value := " padded "
			capture.Details.Friction.CurrentWorkaround = &value
		}, wantField: "current workaround"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capture := valid
			friction := *valid.Details.Friction
			capture.Details.Friction = &friction
			tt.mutate(&capture)
			var invalid *InvalidValueError
			if err := capture.Validate(); !errors.As(err, &invalid) || invalid.Field != tt.wantField {
				t.Fatalf("Validate() error = %T %v, want invalid %s", err, err, tt.wantField)
			}
		})
	}
}

func validProposedFrictionInput() ProposedCaptureInput {
	return ProposedCaptureInput{
		Type:        CaptureTypeFriction,
		Description: "Recurring problem",
		Details: ProposedCaptureDetailsInput{Friction: &FrictionCaptureInput{
			Frequency: FrequencyUnknown,
			Impact:    ImpactUnknown,
			Category:  CategoryOther,
		}},
	}
}
