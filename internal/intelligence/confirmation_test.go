package intelligence

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfirmationPolicy_WriteAlwaysRequiresConfirmation(t *testing.T) {
	policy := DefaultConfirmationPolicy(0.85)
	intent := &ParsedIntent{
		Intent:     IntentProjectRemove,
		Risk:       RiskWrite,
		Confidence: 0.99,
	}
	assert.Equal(t, StateNeedsConfirmation, policy.Evaluate(intent))
}

func TestConfirmationPolicy_WriteIntentEvenIfMarkedReadOnly(t *testing.T) {
	// LLM might incorrectly mark a write intent as read_only.
	// The policy should still catch it via IsWriteIntent.
	policy := DefaultConfirmationPolicy(0.85)
	intent := &ParsedIntent{
		Intent:     IntentProjectAdd,
		Risk:       RiskReadOnly, // LLM error
		Confidence: 0.99,
	}
	assert.Equal(t, StateNeedsConfirmation, policy.Evaluate(intent))
}

func TestConfirmationPolicy_ReadOnlyAutoExecutesAboveThreshold(t *testing.T) {
	policy := DefaultConfirmationPolicy(0.85)
	intent := &ParsedIntent{
		Intent:     IntentWhatNow,
		Risk:       RiskReadOnly,
		Confidence: 0.90,
	}
	assert.Equal(t, StateExecuted, policy.Evaluate(intent))
}

func TestConfirmationPolicy_ReadOnlyAtExactThreshold(t *testing.T) {
	policy := DefaultConfirmationPolicy(0.85)
	intent := &ParsedIntent{
		Intent:     IntentStatus,
		Risk:       RiskReadOnly,
		Confidence: 0.85,
	}
	assert.Equal(t, StateExecuted, policy.Evaluate(intent))
}

func TestConfirmationPolicy_ReadOnlyBelowThresholdNeedsClarification(t *testing.T) {
	policy := DefaultConfirmationPolicy(0.85)
	intent := &ParsedIntent{
		Intent:     IntentWhatNow,
		Risk:       RiskReadOnly,
		Confidence: 0.70,
	}
	assert.Equal(t, StateNeedsClarification, policy.Evaluate(intent))
}

func TestEnforceWriteSafety_SetsRiskAndConfirmation(t *testing.T) {
	intent := &ParsedIntent{
		Intent:               IntentProjectAdd,
		Risk:                 RiskReadOnly,
		RequiresConfirmation: false,
	}
	EnforceWriteSafety(intent)
	assert.Equal(t, RiskWrite, intent.Risk)
	assert.True(t, intent.RequiresConfirmation)
}

func TestEnforceWriteSafety_ReadOnlyIntentUnchanged(t *testing.T) {
	intent := &ParsedIntent{
		Intent:               IntentWhatNow,
		Risk:                 RiskReadOnly,
		RequiresConfirmation: false,
	}
	EnforceWriteSafety(intent)
	assert.Equal(t, RiskReadOnly, intent.Risk)
	assert.False(t, intent.RequiresConfirmation)
}

func TestIsWriteIntent(t *testing.T) {
	writeIntentsList := []IntentName{
		IntentReplan, IntentProjectAdd, IntentProjectImport, IntentProjectUpdate,
		IntentProjectArchive, IntentProjectRemove,
		IntentNodeAdd, IntentNodeUpdate, IntentNodeRemove,
		IntentWorkAdd, IntentWorkUpdate, IntentWorkDone, IntentWorkRemove,
		IntentSessionLog, IntentSessionRemove,
		IntentTemplateDraft, IntentTemplateValidate,
		IntentProjectInitFromTmpl,
	}
	for _, intent := range writeIntentsList {
		assert.True(t, IsWriteIntent(intent), "expected %s to be write", intent)
	}

	readIntentsList := []IntentName{
		IntentWhatNow, IntentStatus, IntentExplainNow,
		IntentExplainWhyNot, IntentReviewWeekly, IntentSimulate,
		IntentTemplateList, IntentTemplateShow,
	}
	for _, intent := range readIntentsList {
		assert.False(t, IsWriteIntent(intent), "expected %s to be read_only", intent)
	}
}
