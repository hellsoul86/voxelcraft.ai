package model

type ContractState string

const (
	ContractOpen      ContractState = "OPEN"
	ContractAccepted  ContractState = "ACCEPTED"
	ContractCompleted ContractState = "COMPLETED"
	ContractFailed    ContractState = "FAILED"
)

type Contract struct {
	ContractID  string
	TerminalPos Vec3i
	Poster      string
	Acceptor    string

	Kind         string
	Requirements map[string]int
	Reward       map[string]int
	Deposit      map[string]int

	// BUILD contracts:
	BlueprintID string
	Anchor      Vec3i
	Rotation    int

	CreatedTick  uint64
	DeadlineTick uint64
	State        ContractState
}

