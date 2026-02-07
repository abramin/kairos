package intelligence

import "fmt"

// ValidateIntentArguments validates the arguments map against the schema
// for the given intent. Returns a ParsedIntentError on failure.
func ValidateIntentArguments(intent IntentName, args map[string]interface{}) *ParsedIntentError {
	validator, ok := intentArgValidators[intent]
	if !ok {
		// Intents without specific schemas pass with empty or nil args.
		return nil
	}
	return validator(args)
}

type argValidator func(map[string]interface{}) *ParsedIntentError

var intentArgValidators = map[IntentName]argValidator{
	IntentWhatNow:           validateWhatNowArgs,
	IntentStatus:            validateStatusArgs,
	IntentReplan:            validateReplanArgs,
	IntentProjectAdd:        validateProjectAddArgs,
	IntentProjectUpdate:     validateProjectUpdateArgs,
	IntentProjectArchive:    validateRequireProjectID,
	IntentProjectRemove:     validateRequireProjectID,
	IntentSessionLog:        validateSessionLogArgs,
	IntentExplainNow:        validateExplainNowArgs,
	IntentExplainWhyNot:     validateExplainWhyNotArgs,
	IntentSimulate:          validateSimulateArgs,
	IntentTemplateDraft:     validateTemplateDraftArgs,
	IntentProjectInitFromTmpl: validateProjectInitArgs,
}

func argError(msg string) *ParsedIntentError {
	return &ParsedIntentError{
		Code:    ErrCodeArgSchemaMismatch,
		Message: msg,
	}
}

func validateWhatNowArgs(args map[string]interface{}) *ParsedIntentError {
	min, ok := getNumber(args, "available_min")
	if !ok {
		return argError("available_min is required and must be a positive number")
	}
	if min <= 0 {
		return argError("available_min must be > 0")
	}
	return nil
}

func validateStatusArgs(args map[string]interface{}) *ParsedIntentError {
	// All fields optional.
	return nil
}

func validateReplanArgs(args map[string]interface{}) *ParsedIntentError {
	if s, ok := args["strategy"]; ok {
		str, isStr := s.(string)
		if !isStr || (str != "rebalance" && str != "deadline_first") {
			return argError("strategy must be 'rebalance' or 'deadline_first'")
		}
	}
	return nil
}

func validateProjectAddArgs(args map[string]interface{}) *ParsedIntentError {
	if _, ok := getString(args, "name"); !ok {
		return argError("name is required for project_add")
	}
	return nil
}

func validateProjectUpdateArgs(args map[string]interface{}) *ParsedIntentError {
	if _, ok := getString(args, "project_id"); !ok {
		return argError("project_id is required for project_update")
	}
	return nil
}

func validateRequireProjectID(args map[string]interface{}) *ParsedIntentError {
	if _, ok := getString(args, "project_id"); !ok {
		return argError("project_id is required")
	}
	return nil
}

func validateSessionLogArgs(args map[string]interface{}) *ParsedIntentError {
	if _, ok := getString(args, "work_item_id"); !ok {
		return argError("work_item_id is required for session_log")
	}
	min, ok := getNumber(args, "minutes")
	if !ok || min <= 0 {
		return argError("minutes is required and must be > 0 for session_log")
	}
	return nil
}

func validateExplainNowArgs(args map[string]interface{}) *ParsedIntentError {
	// minutes is optional.
	if v, exists := args["minutes"]; exists {
		if min, ok := toNumber(v); !ok || min <= 0 {
			return argError("minutes must be > 0 if provided")
		}
	}
	return nil
}

func validateExplainWhyNotArgs(args map[string]interface{}) *ParsedIntentError {
	// At least one identifier should be present.
	_, hasProject := getString(args, "project_id")
	_, hasWorkItem := getString(args, "work_item_id")
	_, hasTitle := getString(args, "candidate_title")
	if !hasProject && !hasWorkItem && !hasTitle {
		return argError("at least one of project_id, work_item_id, or candidate_title is required for explain_why_not")
	}
	return nil
}

func validateSimulateArgs(args map[string]interface{}) *ParsedIntentError {
	if _, ok := getString(args, "scenario_text"); !ok {
		return argError(fmt.Sprintf("scenario_text is required for simulate"))
	}
	return nil
}

func validateTemplateDraftArgs(args map[string]interface{}) *ParsedIntentError {
	if _, ok := getString(args, "prompt"); !ok {
		return argError("prompt is required for template_draft")
	}
	return nil
}

func validateProjectInitArgs(args map[string]interface{}) *ParsedIntentError {
	if _, ok := getString(args, "template_id"); !ok {
		return argError("template_id is required for project_init_from_template")
	}
	if _, ok := getString(args, "project_name"); !ok {
		return argError("project_name is required for project_init_from_template")
	}
	if _, ok := getString(args, "start_date"); !ok {
		return argError("start_date is required for project_init_from_template")
	}
	return nil
}

// helper functions for type-safe argument extraction

func getString(args map[string]interface{}, key string) (string, bool) {
	v, ok := args[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok && s != ""
}

func getNumber(args map[string]interface{}, key string) (float64, bool) {
	v, ok := args[key]
	if !ok {
		return 0, false
	}
	return toNumber(v)
}

func toNumber(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}
