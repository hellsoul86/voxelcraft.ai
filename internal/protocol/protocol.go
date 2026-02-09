package protocol

import "encoding/json"

const Version = "0.9"

// Message types.
const (
	TypeHello   = "HELLO"
	TypeWelcome = "WELCOME"
	TypeCatalog = "CATALOG"
	TypeObs     = "OBS"
	TypeAct     = "ACT"
)

// BaseMessage lets us route unknown JSON messages by type.
type BaseMessage struct {
	Type            string `json:"type"`
	ProtocolVersion string `json:"protocol_version,omitempty"`
}

func DecodeBase(b []byte) (BaseMessage, error) {
	var m BaseMessage
	err := json.Unmarshal(b, &m)
	return m, err
}
