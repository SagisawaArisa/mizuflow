package constraints

type Action int32

const (
	DELETE Action = 0
	PUT    Action = 1
)

const (
	TypeBool   = "bool"
	TypeString = "string"
	TypeJSON   = "json"
	// TypeStrategy indicates a feature flag that uses strategies for evaluation
	TypeStrategy = "strategy"
	TypeNumber   = "number"
)
