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
You may request more context before returning final modules JSON.

Round: %d

Your goal in this round is to identify high-signal files, functions, and event-related flows worth exploring next.
Do not make strong system-level claims unless later evidence supports them.

Available request types:
- read_file {"target":"path/to/file.go"}
- read_function {"target":"path/to/file.go#Name"}
- read_function {"target":"path/to/file.go#Receiver#Method"}
- call_chain {"function_ref":"path/to/file.go#Name","direction":"downstream|upstream","depth":N,"include_events":bool}
- call_chain {"function_ref":"path/to/file.go#Receiver#Method","direction":"downstream|upstream","depth":N,"include_events":bool}
- event_flow {"event_name":"name","expand_publishers":bool,"expand_handlers":bool,"handler_depth":N}

Method line format:
<FuncID> | <Signature> | loc=<n> out=<n> depth=<n|-1> reach=<n> exported=<0|1> entry=<0|1> imp=<0.00-1.00>

Field meanings:
- FuncID: exact queryable function reference.
- loc: lines of code.
- out: internal outbound call count.
- depth: distance from an inferred entry point; -1 means unreachable.
- reach: number of inferred entry points that can reach this function.
- exported: whether the function is public/exported.
- entry: whether the function is an inferred entry point.
- imp: normalized importance score.

Event landmark format:
publish <EventName> @ <FuncID>
handle <EventName> @ <FuncID>
register <EventName> @ <FuncID>

Event landmarks are grounded event facts, not semantic summaries.

How to use these signals:
- Prioritize functions with entry=1, high imp, high out, high reach, or event landmarks.
- Treat metrics as structural prioritization hints, not semantic conclusions.
- Do not infer repository kind, business domains, or architecture style from metrics alone.
- Use read_file for broader file context.
- Use read_function and call_chain only with exact FuncID values shown in the skeleton.

Rules:
- If you still need evidence, return JSON with: {"round":N,"understanding":"...","requests":[...]}
- If you are ready to finish, return JSON with: {"round":N,"modules":[...],"navigation":{"sections":[...]}}
- Terminal payload must use top-level ` + "`modules`" + ` and optional top-level ` + "`navigation`" + `.
- Terminal ` + "`navigation`" + ` maps to ` + "`store.Navigation`" + ` subtree only.
- Terminal ` + "`navigation`" + ` must not wrap another ` + "`modules`" + ` object and must not be presented as a full NavPlan.
- Every file must appear in exactly one module.
- Modules are always required in terminal payload, even when navigation.sections is present.
- Every module must include owner.
- Shared modules must have owner: "shared_preprocessor".
- Non-shared modules must have owner: "agent".
- owner must never be null.
- owner must be one of: "agent" or "shared_preprocessor".
- Allowed navigation section types: generated, business, events, architecture, modules, api.
- Modules may include navigation_refs to section types.
- Shared utilities are files used by %d+ modules.
- Respond ONLY with valid JSON. No markdown fences or prose.

Navigation schema:
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

	sb.WriteString(`{"round":N,"understanding":"...","requests":[{"type":"call_chain","params":{"function_ref":"path/to/file.go#Name","direction":"downstream","depth":2}}]}`)
	sb.WriteString("\n")
	sb.WriteString(`{"round":N,"modules":[{"id":"module","files":["path/to/file.go"],"shared":false,"owner":"agent","depends_on_shared":[],"referenced_by":[],"navigation_refs":["generated","modules"]}],"navigation":{"sections":[{"type":"generated","title":"Generated Overview","description":"...","items":[{"title":"Entry","path":"docs/modules/module.md","entry_point":"path/to/file.go#Name","events":["domain.event"],"highlights":["important detail"]}]},{"type":"modules","title":"Modules","items":[{"title":"Module","path":"docs/modules/module.md"}]}]}}`)
	sb.WriteString("\n\nSkeleton:\n")
	sb.WriteString(contextState.Skeleton)

	return sb.String()
}
