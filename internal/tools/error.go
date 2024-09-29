package gossiper

// ServiceError represents a custom error used throughout the application.
type ServiceError struct {
	Message string // The error message
}

// Error returns the error message, making ServiceError comply with the error interface.
func (e *ServiceError) Error() string {
	return e.Message
}

// GetError returns a map representing the error name and message.
func (e *ServiceError) GetError() map[string]string {
	return map[string]string{
		"name":    "ServiceError",
		"message": e.Message,
	}
}

// NewServiceError creates a new ServiceError instance with the given message.
func NewServiceError(message string) *ServiceError {
	return &ServiceError{
		Message: message,
	}
}
