package planner

import "fmt"

func buildPlannerPrompt(skeleton string, threshold int) string {
	return fmt.Sprintf(`You are a software architect. Given this repository skeleton, group the files
into logical documentation modules. Identify shared utilities used by %d+ modules.

Rules:
- Every file must appear in exactly one module.
- Shared modules must have owner: "shared_preprocessor".
- Respond ONLY with valid JSON. No preamble.

Schema: { modules: [{ id, files[], shared, owner, depends_on_shared[] }] }

Skeleton:
%s`, threshold, skeleton)
}
