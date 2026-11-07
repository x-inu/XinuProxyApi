package auth

// EmailRequiredError indicates that the calling context must provide an email or alias.
type EmailRequiredError struct {
	Prompt string
}

func (e *EmailRequiredError) Error() string {
	if e == nil || e.Prompt == "" {
		return "xinugate auth: email is required"
	}
	return e.Prompt
}
