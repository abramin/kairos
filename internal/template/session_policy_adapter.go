package template

// MinSessionValue returns min_session_min for shared generation policy resolution.
func (sp *SessionPolicyConfig) MinSessionValue() *int {
	if sp == nil {
		return nil
	}
	return sp.MinSessionMin
}

// MaxSessionValue returns max_session_min for shared generation policy resolution.
func (sp *SessionPolicyConfig) MaxSessionValue() *int {
	if sp == nil {
		return nil
	}
	return sp.MaxSessionMin
}

// DefaultSessionValue returns default_session_min for shared generation policy resolution.
func (sp *SessionPolicyConfig) DefaultSessionValue() *int {
	if sp == nil {
		return nil
	}
	return sp.DefaultSessionMin
}

// SplittableValue returns splittable for shared generation policy resolution.
func (sp *SessionPolicyConfig) SplittableValue() *bool {
	if sp == nil {
		return nil
	}
	return sp.Splittable
}
