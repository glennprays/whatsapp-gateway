package httperror

import (
	"errors"
	"net/http"

	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
)

type APIError struct {
	Status  int
	Message string
}

func FromError(err error) APIError {
	var apiError APIError
	var svcError errDomain.Error

	if errors.As(err, &svcError) {
		apiError.Message = svcError.AppError().Error()
		svcErr := svcError.ServiceError()
		switch svcErr {
		case errDomain.ErrBadRequest:
			apiError.Status = http.StatusBadRequest
		case errDomain.ErrInternalFailure:
			apiError.Status = http.StatusInternalServerError
		case errDomain.ErrNotFound:
			apiError.Status = http.StatusNotFound
		case errDomain.ErrUnauthorized:
			apiError.Status = http.StatusUnauthorized
		case errDomain.ErrForbidden:
			apiError.Status = http.StatusForbidden
		case errDomain.ErrConflict:
			apiError.Status = http.StatusConflict
		case errDomain.ErrMethodNotAllowed:
			apiError.Status = http.StatusMethodNotAllowed
		case errDomain.ErrTooManyRequests:
			apiError.Status = http.StatusTooManyRequests
		}
	} else {
		apiError.Message = err.Error()
		apiError.Status = http.StatusInternalServerError
	}

	return apiError
}
