package cli

import (
	"strconv"
	"strings"

	"github.com/damienomurchu/forge-cli/internal/domain"
	"github.com/damienomurchu/forge-cli/internal/repository"
)

type unifiedListRequest struct {
	filters repository.CaptureFilters
	json    bool
}

type unifiedListParseState struct {
	request                       unifiedListRequest
	typeSet, projectSet, limitSet bool
	jsonSet                       bool
}

func parseUnifiedListRequest(args []string) (unifiedListRequest, error) {
	state := unifiedListParseState{}
	for index := 0; index < len(args); index++ {
		argument := args[index]
		switch {
		case argument == "--json":
			if state.jsonSet {
				return unifiedListRequest{}, duplicateUnifiedListFlag("--json")
			}
			state.request.json, state.jsonSet = true, true
		case argument == "--type":
			value, next, err := unifiedListFlagValue(args, index, "--type", false)
			if err != nil {
				return unifiedListRequest{}, err
			}
			if state.typeSet {
				return unifiedListRequest{}, duplicateUnifiedListFlag("--type")
			}
			captureType, err := domain.ParseCaptureType(value)
			if err != nil {
				return unifiedListRequest{}, err
			}
			state.request.filters.Type, state.typeSet, index = &captureType, true, next
		case strings.HasPrefix(argument, "--type="):
			if state.typeSet {
				return unifiedListRequest{}, duplicateUnifiedListFlag("--type")
			}
			value, err := unifiedListEqualsValue(argument, "--type")
			if err != nil {
				return unifiedListRequest{}, err
			}
			captureType, err := domain.ParseCaptureType(value)
			if err != nil {
				return unifiedListRequest{}, err
			}
			state.request.filters.Type, state.typeSet = &captureType, true
		case argument == "--project":
			value, next, err := unifiedListFlagValue(args, index, "--project", false)
			if err != nil {
				return unifiedListRequest{}, err
			}
			if state.projectSet {
				return unifiedListRequest{}, duplicateUnifiedListFlag("--project")
			}
			project := domain.NormalizeOptionalText(value)
			if project == nil {
				return unifiedListRequest{}, &domain.InvalidValueError{Field: "project", Value: value}
			}
			state.request.filters.Project, state.projectSet, index = project, true, next
		case strings.HasPrefix(argument, "--project="):
			if state.projectSet {
				return unifiedListRequest{}, duplicateUnifiedListFlag("--project")
			}
			value, err := unifiedListEqualsValue(argument, "--project")
			if err != nil {
				return unifiedListRequest{}, err
			}
			project := domain.NormalizeOptionalText(value)
			if project == nil {
				return unifiedListRequest{}, &domain.InvalidValueError{Field: "project", Value: value}
			}
			state.request.filters.Project, state.projectSet = project, true
		case argument == "--limit":
			value, next, err := unifiedListFlagValue(args, index, "--limit", true)
			if err != nil {
				return unifiedListRequest{}, err
			}
			if state.limitSet {
				return unifiedListRequest{}, duplicateUnifiedListFlag("--limit")
			}
			limit, err := parseUnifiedListLimit(value)
			if err != nil {
				return unifiedListRequest{}, err
			}
			state.request.filters.Limit, state.limitSet, index = &limit, true, next
		case strings.HasPrefix(argument, "--limit="):
			if state.limitSet {
				return unifiedListRequest{}, duplicateUnifiedListFlag("--limit")
			}
			value, err := unifiedListEqualsValue(argument, "--limit")
			if err != nil {
				return unifiedListRequest{}, err
			}
			limit, err := parseUnifiedListLimit(value)
			if err != nil {
				return unifiedListRequest{}, err
			}
			state.request.filters.Limit, state.limitSet = &limit, true
		default:
			return unifiedListRequest{}, &UsageError{Argument: argument}
		}
	}
	return state.request, nil
}

func unifiedListFlagValue(args []string, index int, flag string, allowLeadingDash bool) (string, int, error) {
	if index+1 >= len(args) || strings.HasPrefix(args[index+1], "--") ||
		(!allowLeadingDash && strings.HasPrefix(args[index+1], "-")) {
		return "", index, &UsageError{Message: flag + " requires a value"}
	}
	return args[index+1], index + 1, nil
}

func unifiedListEqualsValue(argument, flag string) (string, error) {
	value := strings.TrimPrefix(argument, flag+"=")
	if value == "" {
		return "", &UsageError{Message: flag + " requires a value"}
	}
	return value, nil
}

func parseUnifiedListLimit(value string) (int, error) {
	limit, err := strconv.Atoi(value)
	if err != nil || limit <= 0 {
		return 0, &domain.InvalidValueError{Field: "limit", Value: value}
	}
	return limit, nil
}

func duplicateUnifiedListFlag(flag string) error {
	return &UsageError{Message: flag + " may only be specified once"}
}
