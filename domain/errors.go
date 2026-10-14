package domain

import "fmt"

// HTTPError is the interface for HTTP errors that the framework
// automatically converts to the correct status code.
type HTTPError interface {
	error
	StatusCode() int
	Body() any
}

// --- Client errors (4xx) ---

// BadRequestError represents a 400 error.
type BadRequestError struct {
	Message string
}

func (e *BadRequestError) Error() string   { return e.Message }
func (e *BadRequestError) StatusCode() int { return 400 }
func (e *BadRequestError) Body() any       { return map[string]string{"error": e.Message} }

// UnauthorizedError represents a 401 error.
type UnauthorizedError struct {
	Message string
}

func (e *UnauthorizedError) Error() string   { return e.Message }
func (e *UnauthorizedError) StatusCode() int { return 401 }
func (e *UnauthorizedError) Body() any       { return map[string]string{"error": e.Message} }

// ForbiddenError represents a 403 error.
type ForbiddenError struct {
	Message string
}

func (e *ForbiddenError) Error() string   { return e.Message }
func (e *ForbiddenError) StatusCode() int { return 403 }
func (e *ForbiddenError) Body() any       { return map[string]string{"error": e.Message} }

// ErrForbidden is a reusable allocation-free 403 error.
var ErrForbidden = &ForbiddenError{Message: "forbidden"}

// NotFoundError represents a 404 error.
type NotFoundError struct {
	Message string
}

func (e *NotFoundError) Error() string   { return e.Message }
func (e *NotFoundError) StatusCode() int { return 404 }
func (e *NotFoundError) Body() any       { return map[string]string{"error": e.Message} }

// Is implements errors.Is for NotFoundError so any NotFoundError
// matches domain.ErrNotFound via errors.Is.
func (e *NotFoundError) Is(target error) bool {
	_, ok := target.(*NotFoundError)
	return ok
}

// ConflictError represents a 409 error.
type ConflictError struct {
	Message string
}

func (e *ConflictError) Error() string   { return e.Message }
func (e *ConflictError) StatusCode() int { return 409 }
func (e *ConflictError) Body() any       { return map[string]string{"error": e.Message} }

// UnprocessableEntityError represents a 422 error.
type UnprocessableEntityError struct {
	Message string
}

func (e *UnprocessableEntityError) Error() string   { return e.Message }
func (e *UnprocessableEntityError) StatusCode() int { return 422 }
func (e *UnprocessableEntityError) Body() any       { return map[string]string{"error": e.Message} }

// TooManyRequestsError represents a 429 error.
type TooManyRequestsError struct{}

func (e *TooManyRequestsError) Error() string   { return "rate limit exceeded" }
func (e *TooManyRequestsError) StatusCode() int { return 429 }
func (e *TooManyRequestsError) Body() any       { return map[string]string{"error": "rate limit exceeded"} }

// --- Server errors (5xx) ---

// InternalServerError represents a 500 error.
type InternalServerError struct {
	Err error
}

func (e *InternalServerError) Error() string   { return e.Err.Error() }
func (e *InternalServerError) StatusCode() int { return 500 }
func (e *InternalServerError) Body() any       { return map[string]string{"error": "internal server error"} }
func (e *InternalServerError) Unwrap() error   { return e.Err }

// --- Success responses (2xx) ---

// SuccessResponse represents a success response with a body.
//
// It is returned as error so handlers can use the "return c.OK(data)"
// pattern. The framework intercepts this type in HandleError before
// treating it as a real error — StatusCode() and Body() produce the HTTP response.
type SuccessResponse struct {
	Status int
	Data   any
}

func (e *SuccessResponse) Error() string {
	return fmt.Sprintf("success: %d", e.Status)
}
func (e *SuccessResponse) StatusCode() int { return e.Status }
func (e *SuccessResponse) Body() any       { return e.Data }

// --- Existing ErrInvalidJSON ---

// ErrInvalidJSON is returned when generated JSON parse code fails
// (it returns bool, not error). Allocation-free type for the
// common invalid-parse case.
type ErrInvalidJSON struct{}

func (ErrInvalidJSON) Error() string   { return "nitro: invalid JSON body" }
func (ErrInvalidJSON) StatusCode() int { return 400 }
func (ErrInvalidJSON) Body() any       { return map[string]string{"error": "invalid JSON body"} }

// --- CodedError (errors with a machine-readable code) ---

// CodedError is an HTTP error with a machine-readable code,
// useful for clients that need to handle errors programmatically.
//
// JSON response: {"error": "<message>", "code": "<code>"}
type CodedError struct {
	Status  int
	Code    string
	Message string
	// Err is the underlying error (if any). Set by ErrWrap to enable
	// errors.Is/As chaining.
	Err error
}

func (e *CodedError) Error() string   { return e.Message }
func (e *CodedError) StatusCode() int { return e.Status }
func (e *CodedError) Body() any {
	return map[string]string{"error": e.Message, "code": e.Code}
}

// Unwrap implements errors.Is/As chaining.
func (e *CodedError) Unwrap() error { return e.Err }

// --- Pre-defined Errors ---

// ErrNotFound is a reusable allocation-free 404 error.
var ErrNotFound = &NotFoundError{Message: "not found"}

// --- Cache Errors ---

// ErrCacheMiss is returned when a key is not found in the cache.
type ErrCacheMiss struct{}

func (ErrCacheMiss) Error() string { return "nitro: cache miss" }

// ErrEmptyQueue is returned when the queue is empty.
type ErrEmptyQueue struct{}

func (ErrEmptyQueue) Error() string { return "nitro: queue is empty" }
