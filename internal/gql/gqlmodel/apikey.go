package gqlmodel

import (
	"github.com/bitmagnet-io/bitmagnet/internal/apikey"
)

type APIKeyMutation struct {
	APIKeyService apikey.Service
}
