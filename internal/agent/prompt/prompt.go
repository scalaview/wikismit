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

var (
	ModuleSystemPromptTmp template.Template
	ModuleUserPromptTmp   template.Template
)

func init() {
	ModuleSystemPromptTmp = *template.Must(template.New("module_system_prompt").Parse(ModuleSystemPrompt))
	ModuleUserPromptTmp = *template.Must(template.New("module_user_prompt").Parse(ModuleUserPrompt))
}
