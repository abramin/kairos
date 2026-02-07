package intelligence

// EnforceWriteSafety ensures that any write intent has the correct
// risk and requires_confirmation flags, regardless of what the LLM produced.
// This is a hard safety boundary — LLM output cannot bypass it.
func EnforceWriteSafety(intent *ParsedIntent) {
	if IsWriteIntent(intent.Intent) {
		intent.Risk = RiskWrite
		intent.RequiresConfirmation = true
	}
	// Double-check: if risk is write, always require confirmation.
	if intent.Risk == RiskWrite {
		intent.RequiresConfirmation = true
	}
}
