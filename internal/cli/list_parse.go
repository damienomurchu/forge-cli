package cli

import (
	"strconv"
	"strings"

	"github.com/damienomurchu/forge-cli/internal/domain"
	"github.com/damienomurchu/forge-cli/internal/repository"
)

type listRequest struct {
	filters repository.CaptureFilters
	json    bool
}

type listParseState struct {
	request                       listRequest
	typeSet, projectSet, limitSet bool
	jsonSet                       bool
}

func parseListRequest(args []string) (listRequest, error) {
	state := listParseState{}
	for index := 0; index < len(args); index++ {
		argument := args[index]
		switch {
		case argument == "--json":
			if state.jsonSet {
				return listRequest{}, duplicateListFlag("--json")
			}
			state.request.json, state.jsonSet = true, true
		case argument == "--type":
			value, next, err := listFlagValue(args, index, "--type", false)
			if err != nil {
				return listRequest{}, err
			}
			if state.typeSet {
				return listRequest{}, duplicateListFlag("--type")
			}
			captureType, err := domain.ParseCaptureType(value)
			if err != nil {
				return listRequest{}, err
			}
			state.request.filters.Type, state.typeSet, index = &captureType, true, next
		case strings.HasPrefix(argument, "--type="):
			if state.typeSet {
				return listRequest{}, duplicateListFlag("--type")
			}
			value, err := listEqualsValue(argument, "--type")
			if err != nil {
				return listRequest{}, err
			}
			captureType, err := domain.ParseCaptureType(value)
			if err != nil {
				return listRequest{}, err
			}
			state.request.filters.Type, state.typeSet = &captureType, true
		case argument == "--project":
			value, next, err := listFlagValue(args, index, "--project", false)
			if err != nil {
				return listRequest{}, err
			}
			if state.projectSet {
				return listRequest{}, duplicateListFlag("--project")
			}
			project := domain.NormalizeOptionalText(value)
			if project == nil {
				return listRequest{}, &domain.InvalidValueError{Field: "project", Value: value}
			}
			state.request.filters.Project, state.projectSet, index = project, true, next
		case strings.HasPrefix(argument, "--project="):
			if state.projectSet {
				return listRequest{}, duplicateListFlag("--project")
			}
			value, err := listEqualsValue(argument, "--project")
			if err != nil {
				return listRequest{}, err
			}
			project := domain.NormalizeOptionalText(value)
			if project == nil {
				return listRequest{}, &domain.InvalidValueError{Field: "project", Value: value}
			}
			state.request.filters.Project, state.projectSet = project, true
		case argument == "--limit":
			value, next, err := listFlagValue(args, index, "--limit", true)
			if err != nil {
				return listRequest{}, err
			}
			if state.limitSet {
				return listRequest{}, duplicateListFlag("--limit")
			}
			limit, err := parseListLimit(value)
			if err != nil {
				return listRequest{}, err
			}
			state.request.filters.Limit, state.limitSet, index = &limit, true, next
		case strings.HasPrefix(argument, "--limit="):
			if state.limitSet {
				return listRequest{}, duplicateListFlag("--limit")
			}
			value, err := listEqualsValue(argument, "--limit")
			if err != nil {
				return listRequest{}, err
			}
			limit, err := parseListLimit(value)
			if err != nil {
				return listRequest{}, err
			}
			state.request.filters.Limit, state.limitSet = &limit, true
		default:
			return listRequest{}, &UsageError{Argument: argument}
		}
	}
	return state.request, nil
}

func listFlagValue(args []string, index int, flag string, allowLeadingDash bool) (string, int, error) {
	if index+1 >= len(args) || strings.HasPrefix(args[index+1], "--") ||
		(!allowLeadingDash && strings.HasPrefix(args[index+1], "-")) {
		return "", index, &UsageError{Message: flag + " requires a value"}
	}
	return args[index+1], index + 1, nil
}

func listEqualsValue(argument, flag string) (string, error) {
	value := strings.TrimPrefix(argument, flag+"=")
	if value == "" {
		return "", &UsageError{Message: flag + " requires a value"}
	}
	return value, nil
}

func parseListLimit(value string) (int, error) {
	limit, err := strconv.Atoi(value)
	if err != nil || limit <= 0 {
		return 0, &domain.InvalidValueError{Field: "limit", Value: value}
	}
	return limit, nil
}

func duplicateListFlag(flag string) error {
	return &UsageError{Message: flag + " may only be specified once"}
}
