package xjson

import "github.com/ldm2060/axonhub/internal/objects"

var (
	EmptyJSON            = []byte("{}")
	NullJSON             = []byte("null")
	EmptyArrayJSON       = []byte("[]")
	EmptyJSONRawMessage  = objects.JSONRawMessage(EmptyJSON)
	EmptyArrayRawMessage = objects.JSONRawMessage(EmptyArrayJSON)
	NullJSONRawMessage   = objects.JSONRawMessage(NullJSON)
)
