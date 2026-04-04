package prompt

import (
	_ "embed"
	"text/template"
)

//go:embed module_system_prompt.tmpl
var ModuleSystemPrompt string

type ModuleSystemPromptData struct {
	RepoName string `json:"repo_name"`
	RepoType string `json:"repo_type"`
	Language string `json:"language"`
}

//go:embed module_user_prompt.tmpl
var ModuleUserPrompt string

type ModuleUserPromptData struct {
	ModuleID    string `json:"module_id"`
	Skeleton    string `json:"skeleton"`
	SharedBlock string `json:"shared_block"`
	Language    string `json:"language"`
}

//go:embed function_system_prompt.tmpl
var FunctionSystemPrompt string

//go:embed function_user_prompt.tmpl
var FunctionUserPrompt string

type FunctionStruct struct {
	Path            string                  `json:"path"`
	Receiver        string                  `json:"receiver,omitempty"`
	Name            string                  `json:"name"`
	Src             string                  `json:"src"`
	CalledFunctions []*CalledFunctionStruct `json:"called_functions,omitempty"`
}

type CalledFunctionStruct struct {
	Path     string `json:"path"`
	Receiver string `json:"receiver,omitempty"`
	Name     string `json:"name"`
	Summary  string `json:"summary,omitempty"`
}

type FunctionSystemPromptData struct {
	Functions []FunctionStruct `json:"functions"`
}

var (
	ModuleSystemPromptTmp   *template.Template
	ModuleUserPromptTmp     *template.Template
	FunctionSystemPromptTmp *template.Template
	FunctionUserPromptTmp   *template.Template
)

func init() {
	ModuleSystemPromptTmp = template.Must(template.New("module_system_prompt").Parse(ModuleSystemPrompt))
	ModuleUserPromptTmp = template.Must(template.New("module_user_prompt").Parse(ModuleUserPrompt))
	FunctionSystemPromptTmp = template.Must(template.New("function_system_prompt").Parse(FunctionSystemPrompt))
	FunctionUserPromptTmp = template.Must(template.New("function_user_prompt").Parse(FunctionUserPrompt))
}
