package errors

import (
	"net/http"
)

// ServiceError represents a custom error used throughout the application.
type ServiceError struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
}

// Error returns the error message, making ServiceError comply with the error interface.
func (e *ServiceError) Error() string {
	return e.Message
}

// GetError returns a map representing the error status code and message.
func (e *ServiceError) GetError() map[string]interface{} {
	return map[string]any{
		"message":    e.Message,
		"statusCode": e.StatusCode,
	}
}

// NewServiceError creates a new ServiceError instance with the given message and status code.
func NewServiceError(message string, statusCode ...int) *ServiceError {
	code := http.StatusInternalServerError
	if len(statusCode) > 0 {
		code = statusCode[0]
	}
	return &ServiceError{
		Message:    message,
		StatusCode: code,
	}
}
