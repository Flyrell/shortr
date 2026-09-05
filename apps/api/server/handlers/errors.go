package handlers

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v3"

	"github.com/Flyrell/shortr/apps/api/server/services"
)

type errorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

type apiError struct {
	code    string
	message string
	status  int
}

func (e apiError) Error() string { return e.code }

var (
	errInvalidBody          = apiError{status: fiber.StatusBadRequest, code: "invalid_body", message: "the body must be a JSON object with a url field"}
	errForbidden            = apiError{status: fiber.StatusForbidden, code: "forbidden", message: "access to this resource is not allowed"}
	errNotFound             = apiError{status: fiber.StatusNotFound, code: "not_found", message: "the requested resource does not exist"}
	errMethodNotAllowed     = apiError{status: fiber.StatusMethodNotAllowed, code: "method_not_allowed", message: "the method is not allowed for this resource"}
	errBodyTooLarge         = apiError{status: fiber.StatusRequestEntityTooLarge, code: "body_too_large", message: "the request body is too large"}
	errUnsupportedMediaType = apiError{status: fiber.StatusUnsupportedMediaType, code: "unsupported_media_type", message: "the content type must be application/json"}
	errRateLimited          = apiError{status: fiber.StatusTooManyRequests, code: "rate_limited", message: "too many requests, retry later"}
	errInternal             = apiError{status: fiber.StatusInternalServerError, code: "internal", message: "the request could not be completed"}
	errUnavailable          = apiError{status: fiber.StatusServiceUnavailable, code: "unavailable", message: "the service is not ready"}
)

var statusErrors = map[int]apiError{
	errForbidden.status:            errForbidden,
	errNotFound.status:             errNotFound,
	errMethodNotAllowed.status:     errMethodNotAllowed,
	errBodyTooLarge.status:         errBodyTooLarge,
	errUnsupportedMediaType.status: errUnsupportedMediaType,
	errRateLimited.status:          errRateLimited,
	errInternal.status:             errInternal,
	errUnavailable.status:          errUnavailable,
}

func ErrorHandler(logger *slog.Logger) fiber.ErrorHandler {
	return func(c fiber.Ctx, err error) error {
		mapped := mapError(err)
		if mapped.status >= fiber.StatusInternalServerError {
			logger.LogAttrs(c.Context(), slog.LevelError, "request failed",
				slog.String("method", c.Method()),
				slog.String("path", c.Path()),
				slog.Int("status", mapped.status),
				slog.String("error", err.Error()),
			)
		}
		return c.Status(mapped.status).JSON(errorBody{Error: mapped.code, Message: mapped.message})
	}
}

func mapError(err error) apiError {
	var api apiError
	if errors.As(err, &api) {
		return api
	}
	var invalidURL services.InvalidURLError
	switch {
	case errors.As(err, &invalidURL):
		return apiError{status: fiber.StatusBadRequest, code: "invalid_url", message: invalidURL.Message}
	case errors.Is(err, services.ErrNotFound):
		return errNotFound
	}
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		if mapped, ok := statusErrors[fiberErr.Code]; ok {
			return mapped
		}
		return apiError{status: fiberErr.Code, code: "request_failed", message: "the request could not be completed"}
	}
	return errInternal
}
