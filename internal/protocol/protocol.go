package protocol

import "encoding/json"

const Version = "1.1"

var SupportedVersions = map[string]struct{}{
	"0.9": {},
	"1.0": {},
	"1.1": {},
}

func IsSupportedVersion(v string) bool {
	_, ok := SupportedVersions[v]
	return ok
}

// Message types.
const (
	TypeHello         = "HELLO"
	TypeWelcome       = "WELCOME"
	TypeCatalog       = "CATALOG"
	TypeObs           = "OBS"
	TypeAct           = "ACT"
	TypeAck           = "ACK"
	TypeEventBatchReq = "EVENT_BATCH_REQ"
	TypeEventBatch    = "EVENT_BATCH"
)

var preferredVersions = []string{"1.1", "1.0", "0.9"}

func SelectVersion(clientSupported []string, fallback string) (string, bool) {
	seen := map[string]struct{}{}
	for _, v := range clientSupported {
		if _, ok := SupportedVersions[v]; ok {
			seen[v] = struct{}{}
		}
	}
	if fallback != "" {
		if _, ok := SupportedVersions[fallback]; ok {
			seen[fallback] = struct{}{}
		}
	}
	for _, v := range preferredVersions {
		if _, ok := seen[v]; ok {
			return v, true
		}
	}
	return "", false
}

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
