package gossiper

type ServiceError struct {
	Message string
}

func (e *ServiceError) Error() string {
	return e.Message
}

func (e *ServiceError) GetError() map[string]string {
	return map[string]string{
		"name":    "ServiceError",
		"message": e.Message,
	}
}

// NewServiceError Constructor function
func NewServiceError(message string) *ServiceError {
	return &ServiceError{
		Message: message,
	}
}
