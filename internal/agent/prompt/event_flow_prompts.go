package prompt

import (
	_ "embed"
	"text/template"
)

//go:embed event_flow_system_prompt.tmpl
var EventFlowSystemPrompt string

type EventFlowSystemPromptData struct {
	Language string `json:"language"`
}

//go:embed event_flow_user_prompt.tmpl
var EventFlowUserPrompt string

type EventFlowFunctionStruct struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Receiver  string `json:"receiver,omitempty"`
	Name      string `json:"name"`
	Signature string `json:"signature"`
	Summary   string `json:"summary,omitempty"`
	Src       string `json:"src"`
	CalledFunctions []*CalledFunctionStruct `json:"called_functions,omitempty"`
}

type EventFlowUserPromptData struct {
	Functions []*EventFlowFunctionStruct `json:"functions"`
}

var (
	EventFlowSystemPromptTmp *template.Template
	EventFlowUserPromptTmp   *template.Template
)

func init() {
	EventFlowSystemPromptTmp = template.Must(template.New("event_flow_system_prompt").Parse(EventFlowSystemPrompt))
	EventFlowUserPromptTmp = template.Must(template.New("event_flow_user_prompt").Parse(EventFlowUserPrompt))
}
