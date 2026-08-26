package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/damienomurchu/forge-cli/internal/domain"
	"github.com/damienomurchu/forge-cli/internal/output"
)

// collectCapture collects and confirms an interactive capture without
// generating persistence metadata or accessing storage.
func collectCapture(
	ctx context.Context,
	request captureRequest,
	prompter Prompt,
	summaryWriter io.Writer,
) (domain.ProposedCapture, bool, error) {
	if request.quick || request.proposed != nil {
		return domain.ProposedCapture{}, false, errors.New("interactive capture request is required")
	}
	description, err := domain.NormalizeDescription(request.description)
	if err != nil {
		return domain.ProposedCapture{}, false, err
	}
	if prompter == nil {
		return domain.ProposedCapture{}, false, errors.New("capture prompt is required")
	}
	if summaryWriter == nil {
		return domain.ProposedCapture{}, false, errors.New("capture summary writer is required")
	}

	selectedType, err := prompter.Select(
		ctx,
		"Type",
		captureTypeChoices(),
		domain.CaptureTypeFriction.String(),
	)
	if err != nil {
		return domain.ProposedCapture{}, false, capturePromptError("select type for", err)
	}
	captureType, err := domain.ParseCaptureType(selectedType)
	if err != nil {
		return domain.ProposedCapture{}, false, fmt.Errorf("validate selected capture type: %w", err)
	}

	input := domain.ProposedCaptureInput{Type: captureType, Description: description}
	switch captureType {
	case domain.CaptureTypeFriction:
		friction, err := collectFrictionCaptureInput(ctx, prompter)
		if err != nil {
			return domain.ProposedCapture{}, false, err
		}
		input.Details.Friction = &friction
	case domain.CaptureTypeAction:
		input.Details.Action = &domain.ActionCaptureDetails{}
	case domain.CaptureTypeFollowUp:
		input.Details.FollowUp = &domain.FollowUpCaptureDetails{}
	case domain.CaptureTypeDecision:
		input.Details.Decision = &domain.DecisionCaptureDetails{}
	}

	proposed, err := domain.NewProposedCapture(input)
	if err != nil {
		return domain.ProposedCapture{}, false, fmt.Errorf("validate interactive capture: %w", err)
	}
	if err := output.WriteProposedCaptureSummary(summaryWriter, proposed); err != nil {
		return domain.ProposedCapture{}, false, fmt.Errorf("write capture summary: %w", err)
	}
	confirmed, err := prompter.Confirm(ctx, "Create capture?", true)
	if err != nil {
		return domain.ProposedCapture{}, false, capturePromptError("confirm", err)
	}
	if !confirmed {
		return domain.ProposedCapture{}, false, nil
	}
	return proposed, true, nil
}

func collectFrictionCaptureInput(ctx context.Context, prompter Prompt) (domain.FrictionCaptureInput, error) {
	project, err := prompter.Text(ctx, "Project (optional)")
	if err != nil {
		return domain.FrictionCaptureInput{}, capturePromptError("collect project for", err)
	}
	frequencyValue, err := prompter.Select(
		ctx, "Frequency", frequencyChoices(), domain.FrequencyUnknown.String(),
	)
	if err != nil {
		return domain.FrictionCaptureInput{}, capturePromptError("select frequency for", err)
	}
	frequency, err := domain.ParseFrequency(frequencyValue)
	if err != nil {
		return domain.FrictionCaptureInput{}, fmt.Errorf("validate selected frequency: %w", err)
	}
	impactValue, err := prompter.Select(
		ctx, "Impact", impactChoices(), domain.ImpactUnknown.String(),
	)
	if err != nil {
		return domain.FrictionCaptureInput{}, capturePromptError("select impact for", err)
	}
	impact, err := domain.ParseImpact(impactValue)
	if err != nil {
		return domain.FrictionCaptureInput{}, fmt.Errorf("validate selected impact: %w", err)
	}
	categoryValue, err := prompter.Select(
		ctx, "Category", categoryChoices(), domain.CategoryOther.String(),
	)
	if err != nil {
		return domain.FrictionCaptureInput{}, capturePromptError("select category for", err)
	}
	category, err := domain.ParseCategory(categoryValue)
	if err != nil {
		return domain.FrictionCaptureInput{}, fmt.Errorf("validate selected category: %w", err)
	}
	workaround, err := prompter.Text(ctx, "Current workaround (optional)")
	if err != nil {
		return domain.FrictionCaptureInput{}, capturePromptError("collect current workaround for", err)
	}
	return domain.FrictionCaptureInput{
		Project:           project,
		Frequency:         frequency,
		Impact:            impact,
		Category:          category,
		CurrentWorkaround: workaround,
	}, nil
}

func captureTypeChoices() []string {
	types := domain.CaptureTypes()
	choices := make([]string, len(types))
	for index, captureType := range types {
		choices[index] = captureType.String()
	}
	return choices
}

func frequencyChoices() []string {
	return []string{
		domain.FrequencyDaily.String(),
		domain.FrequencyWeekly.String(),
		domain.FrequencyMonthly.String(),
		domain.FrequencyOccasional.String(),
		domain.FrequencyUnknown.String(),
	}
}

func impactChoices() []string {
	return []string{
		domain.ImpactLow.String(),
		domain.ImpactMedium.String(),
		domain.ImpactHigh.String(),
		domain.ImpactUnknown.String(),
	}
}

func categoryChoices() []string {
	return []string{
		domain.CategoryInformationFinding.String(),
		domain.CategoryRepeatedAction.String(),
		domain.CategoryContextSwitching.String(),
		domain.CategoryRemembering.String(),
		domain.CategoryVerification.String(),
		domain.CategoryWaiting.String(),
		domain.CategoryOther.String(),
	}
}
