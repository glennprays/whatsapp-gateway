package error

import "errors"

var (
	ErrBadRequest       = errors.New("bad request")
	ErrNotFound         = errors.New("not found")
	ErrInternalFailure  = errors.New("internal failure")
	ErrUnauthorized     = errors.New("unauthorized")
	ErrForbidden        = errors.New("forbidden")
	ErrConflict         = errors.New("conflict")
	ErrMethodNotAllowed = errors.New("method not allowed")
	ErrTooManyRequests  = errors.New("too many requests")
	ErrGone             = errors.New("gone")
	ErrGatewayTimeout   = errors.New("gateway timeout")
)

type Error struct {
	appErr   error
	svcError error
}

func NewError(svcErr, appErr error) error {
	return Error{
		appErr:   appErr,
		svcError: svcErr,
	}
}

func (e Error) AppError() error {
	return e.appErr
}

func (e Error) ServiceError() error {
	return e.svcError
}

func (e Error) Error() string {
	return errors.Join(e.svcError, e.appErr).Error()
}
