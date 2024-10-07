package tools

import (
	"encoding/json"
	"github.com/rs/zerolog/log"
	"github.com/sirupsen/logrus"
	"os"
)

func (inst *Tools) LogAction(action string, data any) {
	body, err := json.Marshal(data)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal log data")
		return
	}

	log.Info().Str("action", action).Bytes("data", body).Msg("Action logged")
}

var Logger *logrus.Logger

func init() {
	Logger = logrus.New()

	// Set the output to stdout
	Logger.SetOutput(os.Stdout)

	// Set the log level (can be adjusted based on the environment)
	Logger.SetLevel(logrus.InfoLevel)

	// Optionally, set a formatter (e.g., JSON or Text)
	Logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
}

// LogError logs an error message with additional context
func LogError(err error, context string) {
	Logger.WithFields(logrus.Fields{
		"context": context,
	}).Error(err)
}

// LogPanic logs a panic message and recovers from it
func LogPanic(err interface{}) {
	Logger.WithFields(logrus.Fields{
		"panic": err,
	}).Panic("A panic occurred")
}
