package responses

import (
	"errors"
	"net/http"

	internal_errors "github.com/kastoras/images-api/internal/utils/errors"
)

func ErrorResponse(w http.ResponseWriter, err error) {
	var ve *internal_errors.ValidationError
	switch {
	case errors.As(err, &ve):
		BadRequest(w, ve.Error())
	case errors.Is(err, internal_errors.ErrUnsupportedFormat):
		UnsupportedMediaType(w)
	case errors.Is(err, internal_errors.ErrTooManyRequests):
		TooManyRequests(w)
	case errors.Is(err, internal_errors.ErrQueueFull):
		ServiceUnavailable(w, err.Error())
	default:
		InternalServerError(w)
	}
}
