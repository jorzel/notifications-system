package notification

// PermanentError marks a processing failure that retrying cannot fix — an
// invalid recipient, a missing template, or a render error. The worker
// drops these (after logging) instead of retrying. Any other error is
// treated as transient and retried.
type PermanentError struct {
	Err error
}

func newPermanentError(err error) *PermanentError {
	return &PermanentError{Err: err}
}

func (e *PermanentError) Error() string { return e.Err.Error() }

func (e *PermanentError) Unwrap() error { return e.Err }
