package rpc

import "fmt"

// ErrorBuilder provides a fluent API for building errors.
type ErrorBuilder struct {
	code    Code
	message string
	details map[string]any
}

// NewErrorBuilder creates a new ErrorBuilder.
func NewErrorBuilder() *ErrorBuilder {
	return &ErrorBuilder{
		code:    CodeUnknown,
		details: make(map[string]any),
	}
}

// Code sets the error code.
func (e *ErrorBuilder) Code(c Code) *ErrorBuilder {
	e.code = c
	return e
}

// Message sets the error message.
func (e *ErrorBuilder) Message(m string) *ErrorBuilder {
	e.message = m
	return e
}

// Messagef sets the error message with formatting.
func (e *ErrorBuilder) Messagef(format string, args ...any) *ErrorBuilder {
	e.message = fmt.Sprintf(format, args...)
	return e
}

// Detail adds a key-value detail to the error.
func (e *ErrorBuilder) Detail(key string, value any) *ErrorBuilder {
	if e.details == nil {
		e.details = make(map[string]any)
	}
	e.details[key] = value
	return e
}

// Details adds multiple key-value details to the error.
func (e *ErrorBuilder) Details(details map[string]any) *ErrorBuilder {
	if e.details == nil {
		e.details = make(map[string]any)
	}
	for k, v := range details {
		e.details[k] = v
	}
	return e
}

// WithField is an alias for Detail, providing a more intuitive name for validation errors.
func (e *ErrorBuilder) WithField(field string, reason any) *ErrorBuilder {
	return e.Detail(field, reason)
}

// Build creates the final Error.
func (e *ErrorBuilder) Build() *Error {
	err := &Error{
		Code:    e.code,
		Message: e.message,
	}
	if len(e.details) > 0 {
		err.Details = e.details
	}
	return err
}

// Convenience methods for common error types

// InvalidArgument creates an ErrorBuilder with CodeInvalidArgument.
func InvalidArgument(message string) *ErrorBuilder {
	return NewErrorBuilder().Code(CodeInvalidArgument).Message(message)
}

// NotFound creates an ErrorBuilder with CodeNotFound.
func NotFound(message string) *ErrorBuilder {
	return NewErrorBuilder().Code(CodeNotFound).Message(message)
}

// Internal creates an ErrorBuilder with CodeInternal.
func Internal(message string) *ErrorBuilder {
	return NewErrorBuilder().Code(CodeInternal).Message(message)
}

// Unauthenticated creates an ErrorBuilder with CodeUnauthenticated.
func Unauthenticated(message string) *ErrorBuilder {
	return NewErrorBuilder().Code(CodeUnauthenticated).Message(message)
}

// PermissionDenied creates an ErrorBuilder with CodePermissionDenied.
func PermissionDenied(message string) *ErrorBuilder {
	return NewErrorBuilder().Code(CodePermissionDenied).Message(message)
}

// Unimplemented creates an ErrorBuilder with CodeUnimplemented.
func Unimplemented(message string) *ErrorBuilder {
	return NewErrorBuilder().Code(CodeUnimplemented).Message(message)
}

// DeadlineExceeded creates an ErrorBuilder with CodeDeadlineExceeded.
func DeadlineExceeded(message string) *ErrorBuilder {
	return NewErrorBuilder().Code(CodeDeadlineExceeded).Message(message)
}

// ResourceExhausted creates an ErrorBuilder with CodeResourceExhausted.
func ResourceExhausted(message string) *ErrorBuilder {
	return NewErrorBuilder().Code(CodeResourceExhausted).Message(message)
}

// AlreadyExists creates an ErrorBuilder with CodeAlreadyExists.
func AlreadyExists(message string) *ErrorBuilder {
	return NewErrorBuilder().Code(CodeAlreadyExists).Message(message)
}

// Canceled creates an ErrorBuilder with CodeCanceled.
func Canceled(message string) *ErrorBuilder {
	return NewErrorBuilder().Code(CodeCanceled).Message(message)
}

// FailedPrecondition creates an ErrorBuilder with CodeFailedPrecondition.
func FailedPrecondition(message string) *ErrorBuilder {
	return NewErrorBuilder().Code(CodeFailedPrecondition).Message(message)
}

// Aborted creates an ErrorBuilder with CodeAborted.
func Aborted(message string) *ErrorBuilder {
	return NewErrorBuilder().Code(CodeAborted).Message(message)
}

// OutOfRange creates an ErrorBuilder with CodeOutOfRange.
func OutOfRange(message string) *ErrorBuilder {
	return NewErrorBuilder().Code(CodeOutOfRange).Message(message)
}

// DataLoss creates an ErrorBuilder with CodeDataLoss.
func DataLoss(message string) *ErrorBuilder {
	return NewErrorBuilder().Code(CodeDataLoss).Message(message)
}

// Unavailable creates an ErrorBuilder with CodeUnavailable.
func Unavailable(message string) *ErrorBuilder {
	return NewErrorBuilder().Code(CodeUnavailable).Message(message)
}

// ErrorCollector helps collect multiple validation errors.
type ErrorCollector struct {
	errors map[string][]string
}

// NewErrorCollector creates a new ErrorCollector.
func NewErrorCollector() *ErrorCollector {
	return &ErrorCollector{
		errors: make(map[string][]string),
	}
}

// Add adds an error for a field.
func (c *ErrorCollector) Add(field, message string) {
	c.errors[field] = append(c.errors[field], message)
}

// Addf adds a formatted error for a field.
func (c *ErrorCollector) Addf(field, format string, args ...any) {
	c.Add(field, fmt.Sprintf(format, args...))
}

// HasErrors returns true if any errors have been collected.
func (c *ErrorCollector) HasErrors() bool {
	return len(c.errors) > 0
}

// AsError converts the collected errors into an Error.
// Returns nil if no errors have been collected.
func (c *ErrorCollector) AsError() *Error {
	if !c.HasErrors() {
		return nil
	}

	// Build a comprehensive error message
	message := "validation failed"
	if len(c.errors) == 1 {
		for field, msgs := range c.errors {
			if len(msgs) == 1 {
				message = fmt.Sprintf("%s: %s", field, msgs[0])
			} else {
				message = fmt.Sprintf("%s: multiple errors", field)
			}
			break
		}
	} else {
		message = fmt.Sprintf("validation failed for %d fields", len(c.errors))
	}

	// Convert errors map to details
	details := make(map[string]any)
	for field, msgs := range c.errors {
		if len(msgs) == 1 {
			details[field] = msgs[0]
		} else {
			details[field] = msgs
		}
	}

	return &Error{
		Code:    CodeInvalidArgument,
		Message: message,
		Details: details,
	}
}

// Clear removes all collected errors.
func (c *ErrorCollector) Clear() {
	c.errors = make(map[string][]string)
}
