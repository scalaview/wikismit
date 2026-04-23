package agent

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"

	promptpkg "github.com/scalaview/wikismit/internal/agent/prompt"
	"github.com/scalaview/wikismit/pkg/store"
)

func TestEventIndexerBuildAggregatesConfirmedFactsDeterministically(t *testing.T) {
	generatedAt := time.Unix(1710009999, 0).UTC()
	idx := store.FileIndex{
		"internal/handlers/audit.go": {
			Path: "internal/handlers/audit.go",
			Functions: []*store.FunctionDecl{
				{
					Name:     "HandleUserCreated",
					Receiver: "Bus",
					Path:     "internal/handlers/audit.go",
					EventFacts: &store.EventFacts{
						Handles: []*store.EventFact{
							{EventName: "user.created", HandlerRef: "internal/handlers/audit.go#Bus#HandleUserCreated", FuncID: "internal/handlers/audit.go#Bus#HandleUserCreated", Line: 18, Evidence: "case user.created:"},
						},
					},
					EventHints: &store.EventHints{
						LikelyHandles: []*store.EventFact{{EventName: "hint.only", FuncID: "internal/handlers/audit.go#Bus#HandleUserCreated", Line: 19, Evidence: "comment-only hint"}},
					},
				},
				{
					Name:     "RegisterUserCreated",
					Receiver: "Bus",
					Path:     "internal/handlers/audit.go",
					EventFacts: &store.EventFacts{
						Registers: []*store.EventFact{
							{EventName: "user.created", HandlerRef: "internal/handlers/audit.go#Bus#HandleUserCreated", FuncID: "internal/handlers/audit.go#Bus#RegisterUserCreated", Line: 40, Evidence: "bus.Register(user.created, HandleUserCreated)"},
						},
					},
				},
			},
		},
		"internal/service/user.go": {
			Path: "internal/service/user.go",
			Functions: []*store.FunctionDecl{
				{
					Name:     "Create",
					Receiver: "Service",
					Path:     "internal/service/user.go",
					EventFacts: &store.EventFacts{
						Publishes: []*store.EventFact{
							{EventName: "user.created", FuncID: "internal/service/user.go#Service#Create", Line: 87, Evidence: "emit(user.created)"},
							{EventName: "audit.logged", FuncID: "internal/service/user.go#Service#Create", Line: 88, Evidence: "emit(audit.logged)"},
						},
					},
				},
			},
		},
		"internal/api/user.go": {
			Path: "internal/api/user.go",
			Functions: []*store.FunctionDecl{
				{
					Name: "CreateUser",
					Path: "internal/api/user.go",
					EventFacts: &store.EventFacts{
						Publishes: []*store.EventFact{
							{EventName: "user.created", FuncID: "internal/api/user.go#CreateUser", Line: 25, Evidence: "publish after create"},
						},
					},
				},
			},
		},
	}

	got := NewEventIndexer("epic14/v1", func() time.Time { return generatedAt }).Build(idx)
	want := store.EventFactIndex{
		Version:     "epic14/v1",
		GeneratedAt: generatedAt,
		Events: []*store.EventEntry{
			{
				EventName: "audit.logged",
				Publishers: []*store.EventFact{
					{EventName: "audit.logged", FuncID: "internal/service/user.go#Service#Create", Line: 88, Evidence: "emit(audit.logged)"},
				},
			},
			{
				EventName: "user.created",
				Publishers: []*store.EventFact{
					{EventName: "user.created", FuncID: "internal/api/user.go#CreateUser", Line: 25, Evidence: "publish after create"},
					{EventName: "user.created", FuncID: "internal/service/user.go#Service#Create", Line: 87, Evidence: "emit(user.created)"},
				},
				Handlers: []*store.EventFact{
					{EventName: "user.created", HandlerRef: "internal/handlers/audit.go#Bus#HandleUserCreated", FuncID: "internal/handlers/audit.go#Bus#HandleUserCreated", Line: 18, Evidence: "case user.created:"},
				},
				Registrations: []*store.EventFact{
					{EventName: "user.created", HandlerRef: "internal/handlers/audit.go#Bus#HandleUserCreated", FuncID: "internal/handlers/audit.go#Bus#RegisterUserCreated", Line: 40, Evidence: "bus.Register(user.created, HandleUserCreated)"},
				},
			},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Build() = %#v, want %#v", got, want)
	}

	for _, entry := range got.Events {
		if entry.EventName == "hint.only" {
			t.Fatalf("Build() included hint-only event entry: %#v", entry)
		}
	}
	if got.Events[1].Publishers[0].FuncID != "internal/api/user.go#CreateUser" {
		t.Fatalf("Build() publisher ordering = %#v, want internal/api/user.go#CreateUser first", got.Events[1].Publishers)
	}
}

func TestEventFlowPromptTemplatesRenderCompatibleFunctionIdentity(t *testing.T) {
	var systemBuf bytes.Buffer
	if err := promptpkg.EventFlowSystemPromptTmp.Execute(&systemBuf, &promptpkg.EventFlowSystemPromptData{Language: "English"}); err != nil {
		t.Fatalf("EventFlowSystemPromptTmp.Execute() error = %v", err)
	}

	systemMsg := systemBuf.String()
	for _, want := range []string{
		`"summary"`,
		`"event_facts"`,
		`"event_hints"`,
		`"id"`,
		"summary (required, primary)",
		"event_facts (confirmed only)",
		"event_hints (likely_* optional/high-recall)",
		"the function name, or {receiver}#{function_name} when the function has a receiver.",
		`"path"`,
		`"publishes"`,
		`"handles"`,
		`"registers"`,
		`"likely_publishes"`,
		`"likely_handles"`,
		`"likely_registers"`,
	} {
		if !strings.Contains(systemMsg, want) {
			t.Fatalf("EventFlowSystemPromptTmp output missing %q:\n%s", want, systemMsg)
		}
	}

	var userBuf bytes.Buffer
	if err := promptpkg.EventFlowUserPromptTmp.Execute(&userBuf, &promptpkg.EventFlowUserPromptData{
		Functions: []*promptpkg.EventFlowFunctionStruct{{
			ID:        "Service#Create",
			Path:      "internal/service/user.go",
			Receiver:  "Service",
			Name:      "Create",
			Signature: "func (s Service) Create() error",
			Summary:   "Creates a user and emits confirmed lifecycle events.",
			Src:       "func (s Service) Create() error {\n\treturn nil\n}",
			CalledFunctions: []*promptpkg.CalledFunctionStruct{{
				Path:    "internal/events/bus.go",
				Name:    "Publish",
				Summary: "internal/events/bus.go#Publish\nSummary: Publishes a named event to the bus.",
			}},
		}},
	}); err != nil {
		t.Fatalf("EventFlowUserPromptTmp.Execute() error = %v", err)
	}

	userMsg := userBuf.String()
	for _, want := range []string{
		"Path: internal/service/user.go",
		"ID: Service#Create",
		"Receiver: Service",
		"Name: Create",
		"Signature: func (s Service) Create() error",
		"Summary:",
		"Creates a user and emits confirmed lifecycle events.",
		"func (s Service) Create() error {",
		"<called_functions>",
		"internal/events/bus.go#Publish",
	} {
		if !strings.Contains(userMsg, want) {
			t.Fatalf("EventFlowUserPromptTmp output missing %q:\n%s", want, userMsg)
		}
	}
}
