package httperror

import (
	"errors"
	"net/http"

	domain "github.com/glennprays/whatsapp-gateway/domain"
)

type APIError struct {
	Status  int
	Message string
}

func FromError(err error) APIError {
	var apiError APIError
	var svcError domain.Error

	if errors.As(err, &svcError) {
		apiError.Message = svcError.AppError().Error()
		svcErr := svcError.ServiceError()
		switch svcErr {
		case domain.ErrBadRequest:
			apiError.Status = http.StatusBadRequest
		case domain.ErrInternalFailure:
			apiError.Status = http.StatusInternalServerError
		case domain.ErrNotFound:
			apiError.Status = http.StatusNotFound
		case domain.ErrUnauthorized:
			apiError.Status = http.StatusUnauthorized
		case domain.ErrForbidden:
			apiError.Status = http.StatusForbidden
		case domain.ErrConflict:
			apiError.Status = http.StatusConflict
		}
	}

	return apiError
}
