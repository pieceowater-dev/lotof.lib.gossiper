package gossiper

import (
	"encoding/json"
	"github.com/rs/zerolog/log"
)

func (inst *Tools) LogAction(action string, data any) {
	body, err := json.Marshal(data)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal log data")
		return
	}

	log.Info().Str("action", action).Bytes("data", body).Msg("Action logged")
}
