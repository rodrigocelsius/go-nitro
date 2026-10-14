package domain

// Short error constructors for 1-line usage.
//
// Usage in adapters/usecases:
//
//	return NewBadRequest("missing id")
//	return NewInternal(err)
//	return NewNotFound("user")
//	return RespNoContent()

// NewBadRequest returns a 400 error with a custom message.
func NewBadRequest(msg ...string) error {
	m := "bad request"
	if len(msg) > 0 {
		m = msg[0]
	}
	return &BadRequestError{Message: m}
}

// NewUnauthorized returns a 401 error with a custom message.
func NewUnauthorized(msg ...string) error {
	m := "unauthorized"
	if len(msg) > 0 {
		m = msg[0]
	}
	return &UnauthorizedError{Message: m}
}

// NewForbidden returns a 403 error with a custom message.
func NewForbidden(msg ...string) error {
	m := "forbidden"
	if len(msg) > 0 {
		m = msg[0]
	}
	return &ForbiddenError{Message: m}
}

// NewNotFound returns a 404 error with a custom message.
func NewNotFound(msg ...string) error {
	m := "not found"
	if len(msg) > 0 {
		m = msg[0]
	}
	return &NotFoundError{Message: m}
}

// NewConflict returns a 409 error with a custom message.
func NewConflict(msg ...string) error {
	m := "conflict"
	if len(msg) > 0 {
		m = msg[0]
	}
	return &ConflictError{Message: m}
}

// NewUnprocessableEntity returns a 422 error with a custom message.
func NewUnprocessableEntity(msg ...string) error {
	m := "unprocessable entity"
	if len(msg) > 0 {
		m = msg[0]
	}
	return &UnprocessableEntityError{Message: m}
}

// NewInternal returns a 500 error with the original error preserved.
func NewInternal(err error) error {
	return &InternalServerError{Err: err}
}

// NewTooManyRequests returns a 429 error (rate limit).
func NewTooManyRequests() error {
	return &TooManyRequestsError{}
}

// RespOK returns a 200 response with a body.
func RespOK(data any) error {
	return &SuccessResponse{Status: 200, Data: data}
}

// RespCreated returns a 201 response with a body.
func RespCreated(data any) error {
	return &SuccessResponse{Status: 201, Data: data}
}

// RespNoContent returns a 204 response with no body.
func RespNoContent() error {
	return &SuccessResponse{Status: 204}
}
