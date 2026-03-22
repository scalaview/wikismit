package agent

import (
	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/scalaview/wikismit/pkg/store"
)

type AgentInput struct {
	Module        store.Module
	FileIndex     store.FileIndex
	SharedContext store.SharedContext
	Config        *configpkg.Config
}

type ModuleDoc struct {
	ModuleID string
	Content  string
	Err      error
}
