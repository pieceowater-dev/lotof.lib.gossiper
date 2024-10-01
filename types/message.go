package types

// DefaultMessage defines the default message structure for Gossiper
// Pattern is the type or category of the message, Data is the payload
type DefaultMessage struct {
	Pattern string `json:"pattern"`
	Data    any    `json:"data"`
}
