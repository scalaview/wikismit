package planner

import (
	"encoding/json"
	"fmt"
	"strings"
)

func buildPlannerPrompt(skeleton string, threshold int) string {
	return fmt.Sprintf(`You are a software architect. Given this repository skeleton, group the files
into logical documentation modules and richer navigation sections. Identify shared utilities used by %d+ modules.

Rules:
- Every file must appear in exactly one module.
- Modules stay required for compatibility, even when navigation is present.
- Every module must include an owner field.
- Shared modules must have owner: "shared_preprocessor".
- Non-shared modules must have owner: "agent".
- owner must never be null.
- owner must be one of: "agent" or "shared_preprocessor".
- If shared is true, owner must be "shared_preprocessor".
- If shared is false, owner must be "agent".
- Also return version and navigation.sections.
- Allowed navigation section types: generated, business, events, architecture, modules, api.
- navigation groups concepts; modules still own files and coverage.
- Modules may include navigation_refs to the section types they belong to.
- Functions marked with ★ are high-impact. Consider making them module entry points or giving them dedicated documentation sections.
- Respond ONLY with valid JSON. No preamble.

Schema: { version, navigation: { sections: [{ type, title, description, items: [{ title, path, entry_point, events[], highlights[] }] }] }, modules: [{ id, files[], shared, owner, depends_on_shared[], referenced_by[], navigation_refs[] }] }

Example:
{
  "version": "planner/v2",
  "navigation": {
    "sections": [
      {
        "type": "generated",
        "title": "Generated Overview",
        "description": "High-level generated entrypoints",
        "items": [
          {
            "title": "Planner overview",
            "path": "docs/modules/planner.md",
            "entry_point": "internal/planner/planner.go#RunPlanner",
            "highlights": ["Coordinates module planning"]
          }
        ]
      },
      {
        "type": "modules",
        "title": "Modules",
        "items": [
          {
            "title": "Planner module",
            "path": "docs/modules/planner.md"
          }
        ]
      }
    ]
  },
  "modules": [
    {
      "id": "planner",
      "files": ["internal/planner/planner.go"],
      "shared": false,
      "owner": "agent",
      "depends_on_shared": ["config", "llm", "store"],
      "referenced_by": [],
      "navigation_refs": ["generated", "modules"]
    },
    {
      "id": "store",
      "files": ["pkg/store/artifacts.go", "pkg/store/index.go"],
      "shared": true,
      "owner": "shared_preprocessor",
      "depends_on_shared": [],
      "referenced_by": ["planner", "agent"],
      "navigation_refs": ["architecture", "api"]
    }
  ]
}

Skeleton:
%s`, threshold, skeleton)
}

func buildInteractivePlannerPrompt(contextState *PlannerRoundContext, threshold int) string {
	if contextState == nil {
		return buildPlannerPrompt("", threshold)
	}

	var sb strings.Builder
	_, _ = fmt.Fprintf(&sb, `You are a software architect running a bounded multi-round planning protocol.
You may request more context before returning the final modules JSON.

Round: %d

Available request types:
- read_file {"target":"path/to/file.go"}
- read_function {"target":"path/to/file.go#Name"}
- read_function {"target":"path/to/file.go#Receiver#Method"}
- call_chain {"function_ref":"path/to/file.go#Name","direction":"downstream|upstream","depth":N,"include_events":bool}
- call_chain {"function_ref":"path/to/file.go#Receiver#Method","direction":"downstream|upstream","depth":N,"include_events":bool}
- event_flow {"event_name":"name","expand_publishers":bool,"expand_handlers":bool,"handler_depth":N}

Rules:
- If you still need evidence, return JSON with: {"round":N,"understanding":"...","requests":[...]}
- If you are ready to finish, return JSON with: {"round":N,"navigation":{"modules":[...]}}
- Final navigation should include version and may include navigation.sections.
- Final navigation must stay modules-compatible for now.
- Every file must appear in exactly one module.
- Modules stay required for compatibility, even when navigation.sections is present.
- Every module must include owner.
- Shared modules must have owner: "shared_preprocessor".
- Non-shared modules must have owner: "agent".
- owner must never be null.
- owner must be one of: "agent" or "shared_preprocessor".
- Allowed navigation section types: generated, business, events, architecture, modules, api.
- Modules may include navigation_refs to section types.
- Shared utilities are files used by %d+ modules.
- Respond ONLY with valid JSON. No markdown fences or prose.

`, contextState.Round, threshold)

	if contextState.ExplorationContext != "" {
		sb.WriteString("Understanding:\n")
		sb.WriteString(contextState.ExplorationContext)
		sb.WriteString("\n\n")
	}

	if len(contextState.PreviousResponses) > 0 {
		sb.WriteString("Previous responses:\n")
		for _, response := range contextState.PreviousResponses {
			if response == nil {
				continue
			}
			encoded, err := json.Marshal(response.Result)
			if err != nil {
				continue
			}
			sb.WriteString("- ")
			sb.WriteString(response.Type)
			sb.WriteString(": ")
			sb.Write(encoded)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Navigation schema:\n")
	sb.WriteString(`{"version":"planner/v2","navigation":{"sections":[{"type":"generated","title":"Generated Overview","description":"...","items":[{"title":"Entry","path":"docs/modules/module.md","entry_point":"path/to/file.go#Name","events":["domain.event"],"highlights":["important detail"]}]},{"type":"modules","title":"Modules","items":[{"title":"Module","path":"docs/modules/module.md"}]}]},"modules":[{"id":"module","files":["path/to/file.go"],"shared":false,"owner":"agent","depends_on_shared":[],"referenced_by":[],"navigation_refs":["generated","modules"]}]}`)
	sb.WriteString("\n\nSkeleton:\n")
	sb.WriteString(contextState.Skeleton)

	return sb.String()
}
