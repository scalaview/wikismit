package planner

import "encoding/json"

type PlannerRoundRequest struct {
	Round         int               `json:"round"`
	Understanding string            `json:"understanding,omitempty"`
	Requests      []*PlannerRequest `json:"requests,omitempty"`
	Modules       json.RawMessage   `json:"modules,omitempty"`
	Navigation    json.RawMessage   `json:"navigation,omitempty"`
}

type PlannerRequest struct {
	Type   string          `json:"type"`
	Params json.RawMessage `json:"params"`
}

type PlannerRoundContext struct {
	Round              int
	Skeleton           string
	ExplorationContext string
	PreviousResponses  []*PlannerResponseEnvelope
}

type PlannerResponseEnvelope struct {
	Type   string `json:"type"`
	Result any    `json:"result"`
}
