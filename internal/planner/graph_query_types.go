package planner

type CallChainQuery struct {
	FunctionRef   string `json:"function_ref"`
	Direction     string `json:"direction"`
	Depth         int    `json:"depth"`
	IncludeEvents bool   `json:"include_events"`
	MaxNodes      int    `json:"max_nodes,omitempty"`
	MaxEdges      int    `json:"max_edges,omitempty"`
}

type EventFlowQuery struct {
	EventName        string `json:"event_name"`
	ExpandPublishers bool   `json:"expand_publishers"`
	ExpandHandlers   bool   `json:"expand_handlers"`
	HandlerDepth     int    `json:"handler_depth"`
	MaxNodes         int    `json:"max_nodes,omitempty"`
	MaxEdges         int    `json:"max_edges,omitempty"`
}

type FlowNode struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Label string `json:"label,omitempty"`
}

type FlowEdge struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Kind       string `json:"kind"`
	Confidence string `json:"confidence"`
	Source     string `json:"source"`
}

type MissingLink struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Problem string `json:"problem"`
}

type OpenQuestion struct {
	Question string `json:"question"`
	Context  string `json:"context"`
}

type CandidateRead struct {
	Target string `json:"target"`
	Reason string `json:"reason"`
}

type FlowGraphResult struct {
	Nodes          []*FlowNode      `json:"nodes"`
	Edges          []*FlowEdge      `json:"edges"`
	MissingLinks   []*MissingLink   `json:"missing_links,omitempty"`
	OpenQuestions  []*OpenQuestion  `json:"open_questions,omitempty"`
	CandidateReads []*CandidateRead `json:"candidate_reads,omitempty"`
	Truncated      bool             `json:"truncated,omitempty"`
}
