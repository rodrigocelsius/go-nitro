package nitro

import (
	"database/sql"
	"errors"
	"log"

	"github.com/rodrigocelsius/go-nitro/domain/v2"
	"github.com/rodrigocelsius/go-nitro/usecases/v2"
)

// === Error Constructors ===

// firstMsg returns msg[0] if present, otherwise def.
func firstMsg(def string, msg ...string) string {
	if len(msg) > 0 && msg[0] != "" {
		return msg[0]
	}
	return def
}

// wrapAdapter adapts domain.Handler (FastCtx.Ctx) to nitro.Handler.
func wrapAdapter(mw domain.Handler) Handler {
	return func(c *Ctx) error {
		return mw(&c.Ctx)
	}
}

func BadRequest(msg ...string) error {
	return &domain.BadRequestError{Message: firstMsg("bad request", msg...)}
}

func Unauthorized(msg ...string) error {
	return &domain.UnauthorizedError{Message: firstMsg("unauthorized", msg...)}
}

func Forbidden(msg ...string) error {
	return &domain.ForbiddenError{Message: firstMsg("forbidden", msg...)}
}

func NotFound(msg ...string) error {
	return &domain.NotFoundError{Message: firstMsg("not found", msg...)}
}

func Conflict(msg ...string) error {
	return &domain.ConflictError{Message: firstMsg("conflict", msg...)}
}

func UnprocessableEntity(msg ...string) error {
	return &domain.UnprocessableEntityError{Message: firstMsg("unprocessable entity", msg...)}
}

func InternalError(err error) error {
	return &domain.InternalServerError{Err: err}
}

// OKOrNil responds HTTP 200 with body if err == nil or err is a not-found error.
// For other errors, it returns the original error.
//
//	return nitro.OKOrNil(nitro.Map{"driver": d}, err)
func OKOrNil(body any, err error) error {
	if err != nil {
		if isNotFoundErr(err) {
			return &domain.SuccessResponse{Status: 200, Data: body}
		}
		return err
	}
	return &domain.SuccessResponse{Status: 200, Data: body}
}

func isNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, domain.ErrNotFound) {
		return true
	}
	if errors.Is(err, sql.ErrNoRows) {
		return true
	}
	var nf notFounder
	return errors.As(err, &nf) && nf.NotFound()
}

// notFounder allows custom errors to opt into 404 semantics.
type notFounder interface {
	NotFound() bool
}

// Ok wraps Listen — prints the error and exits on failure.
func Ok(err error) {
	if err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// ErrInvalidJSON is the invalid JSON parse error.
type ErrInvalidJSON = domain.ErrInvalidJSON

// === Phase 4: Err (Coded Errors) ===

// CodedError is an HTTP error with a machine-readable code.
type CodedError = domain.CodedError

// Err creates an HTTP error with a machine-readable code.
//
//	return nitro.Err(400, "INVALID_EMAIL", "email field is required")
//	// {"error":"email field is required","code":"INVALID_EMAIL"}
func Err(statusCode int, code, message string) error {
	return &domain.CodedError{
		Status:  statusCode,
		Code:    code,
		Message: message,
	}
}

// ErrCode creates a 400 error with a machine-readable code.
//
//	return nitro.ErrCode("INVALID_EMAIL", "email field is required")
func ErrCode(code, message string) error {
	return Err(400, code, message)
}

// ErrWrap creates an HTTP error with a code, wrapping an existing error.
// The wrapped error is preserved for errors.Is/As via Unwrap. Message stays
// stable (does not embed fmt of the cause).
func ErrWrap(statusCode int, code, message string, err error) error {
	return &domain.CodedError{
		Status:  statusCode,
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// ProblemDetails returns an ErrorHandler that renders errors as RFC 9457
// (RFC 7807) application/problem+json:
//
//	app := nitro.New(nitro.Config{ErrorHandler: nitro.ProblemDetails()})
func ProblemDetails() func(c *domain.Ctx, err error) error {
	return usecases.ProblemDetails()
}
