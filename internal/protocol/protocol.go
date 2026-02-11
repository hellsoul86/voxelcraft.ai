package protocol

import "encoding/json"

const Version = "1.0"

var SupportedVersions = map[string]struct{}{
	"0.9": {},
	"1.0": {},
}

func IsSupportedVersion(v string) bool {
	_, ok := SupportedVersions[v]
	return ok
}

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
