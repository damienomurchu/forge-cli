package cli

import (
	"fmt"
	"strings"

	"github.com/damienomurchu/forge-cli/internal/domain"
)

type captureRequest struct {
	quick       bool
	json        bool
	description string
	proposed    *domain.ProposedCapture
}

type captureParseState struct {
	quick, quickSet      bool
	json, jsonSet        bool
	captureType          domain.CaptureType
	typeSet              bool
	project              string
	projectSet           bool
	frequency            domain.Frequency
	frequencySet         bool
	impact               domain.Impact
	impactSet            bool
	category             domain.Category
	categorySet          bool
	currentWorkaround    string
	currentWorkaroundSet bool
	positionals          []string
}

func parseCaptureRequest(args []string) (captureRequest, error) {
	state := captureParseState{
		frequency: domain.FrequencyUnknown,
		impact:    domain.ImpactUnknown,
		category:  domain.CategoryOther,
	}
	optionsEnded := false
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case !optionsEnded && arg == "--":
			optionsEnded = true
		case !optionsEnded && arg == "--quick":
			if state.quickSet {
				return captureRequest{}, duplicateCaptureFlag("--quick")
			}
			state.quick, state.quickSet = true, true
		case !optionsEnded && arg == "--json":
			if state.jsonSet {
				return captureRequest{}, duplicateCaptureFlag("--json")
			}
			state.json, state.jsonSet = true, true
		case !optionsEnded && arg == "--type":
			value, next, err := captureFlagValue(args, index, "--type")
			if err != nil {
				return captureRequest{}, err
			}
			if state.typeSet {
				return captureRequest{}, duplicateCaptureFlag("--type")
			}
			captureType, err := domain.ParseCaptureType(value)
			if err != nil {
				return captureRequest{}, err
			}
			state.captureType, state.typeSet, index = captureType, true, next
		case !optionsEnded && strings.HasPrefix(arg, "--type="):
			if state.typeSet {
				return captureRequest{}, duplicateCaptureFlag("--type")
			}
			value, err := captureEqualsValue(arg, "--type")
			if err != nil {
				return captureRequest{}, err
			}
			captureType, err := domain.ParseCaptureType(value)
			if err != nil {
				return captureRequest{}, err
			}
			state.captureType, state.typeSet = captureType, true
		case !optionsEnded && arg == "--project":
			value, next, err := captureFlagValue(args, index, "--project")
			if err != nil {
				return captureRequest{}, err
			}
			if state.projectSet {
				return captureRequest{}, duplicateCaptureFlag("--project")
			}
			state.project, state.projectSet, index = value, true, next
		case !optionsEnded && strings.HasPrefix(arg, "--project="):
			if state.projectSet {
				return captureRequest{}, duplicateCaptureFlag("--project")
			}
			state.project = strings.TrimPrefix(arg, "--project=")
			state.projectSet = true
		case !optionsEnded && arg == "--frequency":
			value, next, err := captureFlagValue(args, index, "--frequency")
			if err != nil {
				return captureRequest{}, err
			}
			if state.frequencySet {
				return captureRequest{}, duplicateCaptureFlag("--frequency")
			}
			frequency, err := domain.ParseFrequency(value)
			if err != nil {
				return captureRequest{}, err
			}
			state.frequency, state.frequencySet, index = frequency, true, next
		case !optionsEnded && strings.HasPrefix(arg, "--frequency="):
			if state.frequencySet {
				return captureRequest{}, duplicateCaptureFlag("--frequency")
			}
			value, err := captureEqualsValue(arg, "--frequency")
			if err != nil {
				return captureRequest{}, err
			}
			frequency, err := domain.ParseFrequency(value)
			if err != nil {
				return captureRequest{}, err
			}
			state.frequency, state.frequencySet = frequency, true
		case !optionsEnded && arg == "--impact":
			value, next, err := captureFlagValue(args, index, "--impact")
			if err != nil {
				return captureRequest{}, err
			}
			if state.impactSet {
				return captureRequest{}, duplicateCaptureFlag("--impact")
			}
			impact, err := domain.ParseImpact(value)
			if err != nil {
				return captureRequest{}, err
			}
			state.impact, state.impactSet, index = impact, true, next
		case !optionsEnded && strings.HasPrefix(arg, "--impact="):
			if state.impactSet {
				return captureRequest{}, duplicateCaptureFlag("--impact")
			}
			value, err := captureEqualsValue(arg, "--impact")
			if err != nil {
				return captureRequest{}, err
			}
			impact, err := domain.ParseImpact(value)
			if err != nil {
				return captureRequest{}, err
			}
			state.impact, state.impactSet = impact, true
		case !optionsEnded && arg == "--category":
			value, next, err := captureFlagValue(args, index, "--category")
			if err != nil {
				return captureRequest{}, err
			}
			if state.categorySet {
				return captureRequest{}, duplicateCaptureFlag("--category")
			}
			category, err := domain.ParseCategory(value)
			if err != nil {
				return captureRequest{}, err
			}
			state.category, state.categorySet, index = category, true, next
		case !optionsEnded && strings.HasPrefix(arg, "--category="):
			if state.categorySet {
				return captureRequest{}, duplicateCaptureFlag("--category")
			}
			value, err := captureEqualsValue(arg, "--category")
			if err != nil {
				return captureRequest{}, err
			}
			category, err := domain.ParseCategory(value)
			if err != nil {
				return captureRequest{}, err
			}
			state.category, state.categorySet = category, true
		case !optionsEnded && arg == "--current-workaround":
			value, next, err := captureFlagValue(args, index, "--current-workaround")
			if err != nil {
				return captureRequest{}, err
			}
			if state.currentWorkaroundSet {
				return captureRequest{}, duplicateCaptureFlag("--current-workaround")
			}
			state.currentWorkaround, state.currentWorkaroundSet, index = value, true, next
		case !optionsEnded && strings.HasPrefix(arg, "--current-workaround="):
			if state.currentWorkaroundSet {
				return captureRequest{}, duplicateCaptureFlag("--current-workaround")
			}
			state.currentWorkaround = strings.TrimPrefix(arg, "--current-workaround=")
			state.currentWorkaroundSet = true
		case !optionsEnded && strings.HasPrefix(arg, "-"):
			return captureRequest{}, &UsageError{Argument: arg}
		default:
			state.positionals = append(state.positionals, arg)
		}
	}
	return finalizeCaptureRequest(state)
}

func finalizeCaptureRequest(state captureParseState) (captureRequest, error) {
	if len(state.positionals) == 0 {
		return captureRequest{}, &UsageError{Message: "capture requires a description"}
	}
	if len(state.positionals) > 1 {
		return captureRequest{}, &UsageError{Message: fmt.Sprintf("unexpected argument %q", state.positionals[1])}
	}
	description, err := domain.NormalizeDescription(state.positionals[0])
	if err != nil {
		return captureRequest{}, err
	}
	request := captureRequest{quick: state.quick, json: state.json, description: description}
	frictionFlagsSet := state.projectSet || state.frequencySet || state.impactSet ||
		state.categorySet || state.currentWorkaroundSet
	if !state.quick {
		if state.typeSet {
			return captureRequest{}, &UsageError{Message: "--type requires --quick"}
		}
		if frictionFlagsSet {
			return captureRequest{}, &UsageError{Message: "friction options require --quick --type friction"}
		}
		return request, nil
	}
	if !state.typeSet {
		return captureRequest{}, &UsageError{Message: "--quick requires --type"}
	}
	if state.captureType != domain.CaptureTypeFriction && frictionFlagsSet {
		return captureRequest{}, &UsageError{Message: "friction options require --type friction"}
	}

	input := domain.ProposedCaptureInput{Type: state.captureType, Description: description}
	switch state.captureType {
	case domain.CaptureTypeFriction:
		input.Details.Friction = &domain.FrictionCaptureInput{
			Project:           state.project,
			Frequency:         state.frequency,
			Impact:            state.impact,
			Category:          state.category,
			CurrentWorkaround: state.currentWorkaround,
		}
	case domain.CaptureTypeAction:
		input.Details.Action = &domain.ActionCaptureDetails{}
	case domain.CaptureTypeFollowUp:
		input.Details.FollowUp = &domain.FollowUpCaptureDetails{}
	case domain.CaptureTypeDecision:
		input.Details.Decision = &domain.DecisionCaptureDetails{}
	}
	proposed, err := domain.NewProposedCapture(input)
	if err != nil {
		return captureRequest{}, err
	}
	request.proposed = &proposed
	return request, nil
}

func captureFlagValue(args []string, index int, flag string) (string, int, error) {
	if index+1 >= len(args) || strings.HasPrefix(args[index+1], "-") {
		return "", index, &UsageError{Message: flag + " requires a value"}
	}
	return args[index+1], index + 1, nil
}

func captureEqualsValue(argument, flag string) (string, error) {
	value := strings.TrimPrefix(argument, flag+"=")
	if value == "" {
		return "", &UsageError{Message: flag + " requires a value"}
	}
	return value, nil
}

func duplicateCaptureFlag(flag string) error {
	return &UsageError{Message: flag + " may only be specified once"}
}
