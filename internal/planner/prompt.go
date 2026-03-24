package planner

import "fmt"

func buildPlannerPrompt(skeleton string, threshold int) string {
	return fmt.Sprintf(`You are a software architect. Given this repository skeleton, group the files
into logical documentation modules. Identify shared utilities used by %d+ modules.

Rules:
- Every file must appear in exactly one module.
- Every module must include an owner field.
- Shared modules must have owner: "shared_preprocessor".
- Non-shared modules must have owner: "agent".
- owner must never be null.
- owner must be one of: "agent" or "shared_preprocessor".
- If shared is true, owner must be "shared_preprocessor".
- If shared is false, owner must be "agent".
- Respond ONLY with valid JSON. No preamble.

Schema: { modules: [{ id, files[], shared, owner, depends_on_shared[], referenced_by[] }] }

Example:
{
  "modules": [
    {
      "id": "planner",
      "files": ["internal/planner/planner.go"],
      "shared": false,
      "owner": "agent",
      "depends_on_shared": ["config", "llm", "store"],
      "referenced_by": []
    },
    {
      "id": "store",
      "files": ["pkg/store/artifacts.go", "pkg/store/index.go"],
      "shared": true,
      "owner": "shared_preprocessor",
      "depends_on_shared": [],
      "referenced_by": ["planner", "agent"]
    }
  ]
}

Skeleton:
%s`, threshold, skeleton)
}
