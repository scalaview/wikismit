# wikismit — Epic 9C: Feature Enhancement — Architecture Summary & Cross-Module Context

**Status:** `todo`
**Depends on:** Epic 9D
**Goal:** 解决文档割裂感的根本原因——Planner 产出架构摘要，Agent 获得跨模块上下文，Composer 生成架构总览页。
**Spec refs:** code-review-improvement.md §1.1-§1.3

---

## S9D.1 — Extend NavPlan with Architecture Summary

**Status:** `todo`

**Description:**
当前 `NavPlan` 只有模块分组信息，没有系统架构层面的描述。 Planner 在这一步已经"看过全局骨架"，是生成整体架构描述的最佳时机，但当前直接丢弃了这一上下文。扩展 `NavPlan` 结构体，修改 Planner prompt 让其同时产出 `architecture_summary`。

**Acceptance criteria:**
- `store.NavPlan` 包含 `ArchitectureSummary` 字段
- `store.ArchSummary` 结构体包含 `Purpose`, `Layers`, `DataFlow`, `KeyModules`
- Planner prompt 追加架构摘要输出要求
- 现有 `nav_plan.json` 向后兼容（新字段可选）
- LLM 响应被正确解析为新的结构体

**Files to modify:**
```
pkg/store/artifacts.go
internal/planner/prompt.go
internal/planner/planner.go
pkg/store/artifacts_test.go
```

### Subtasks

#### S9D.1.1 — Define ArchSummary Structure
Legacy - Add `ArchSummary` struct to `pkg/store/artifacts.go` with fields: `Purpose`, `Layers`, `DataFlow`, `KeyModules`
- Add `ArchitectureSummary` field to `NavPlan` with JSON tag `architecture_summary`
- Add roundtrip (marshal/unmarshal) test

#### S9D.1.2 — Extend Planner Prompt
Legacy - Modify `buildPlannerPrompt` to request `architecture_summary` field in the output JSON
- Describe expected fields: purpose (1-2 sentences), layers (architectural layers), data_flow (end-to-end), key_modules (3-5 central modules)
- Keep existing module grouping requirements unchanged

#### S9D.1.3 — Validate Architecture Summary Output
Legacy - Add basic validation for `ArchSummary` in `validateNavPlan` or a separate validator
- At minimum, `Purpose` should be non-empty
- Allow `Layers`, `DataFlow`, `KeyModules` to be empty (graceful degradation)

#### S9D.1.4 — Verify Planner Produces Architecture Summary
Legacy - Run `go test ./internal/planner -v`
- Test that planner output includes `architecture_summary` field
- Re-run `go test ./...`

---

## S9D.2 — Inject Cross-Module Context into Agent Prompts

**Status:** `todo`

**Description:**
每个 Agent 对其他非共享模块一无所知，导致文档孤立。在 `AgentInput` 中增加架构上下文，并在 Agent prompt 头部注入，让每个 Agent 能在整体视角下写文档。

**Acceptance criteria:**
- `AgentInput` 包含 `ArchSummary` 字段
- `AgentInput` 包含 `NeighborSummaries` (map[string]string) 字段
- Agent prompt 头部注入 System Architecture Context 和 This Module's Role 区块
- 现有 agent 输出内容不受破坏性影响（增量改进）

**Files to modify:**
```
internal/agent/types.go
internal/agent/prompt.go
internal/agent/agent.go
internal/pipeline/incremental.go
internal/agent/prompt_test.go
```

### Subtasks

#### S9D.2.1 — Extend AgentInput Structure
Legacy - Add `ArchSummary store.ArchSummary` field to `AgentInput`
- Add `NeighborSummaries map[string]string` field to `AgentInput`
- Populate `NeighborSummaries` from `NavPlan` module relationships (upstream/downstream based on `DependsOnShared`/`ReferencedBy`)

#### S9D.2.2 — Inject Architecture Context into Agent Prompt
Legacy - Add "## System Architecture Context" section at prompt head with purpose, layers, data_flow
- Add "## This Module's Role" section with upstream/downstream module summaries
- Keep existing prompt structure (code skeleton, shared modules block) unchanged

#### S9D.2.3 — Wire Architecture Context through Pipeline
Legacy - Update `RunFullGenerate` and `RunIncremental` to pass `plan.ArchitectureSummary` to `AgentInput`
- Build `NeighborSummaries` from plan modules' dependency relationships
- Ensure backward compatibility if `ArchSummary` is empty

#### S9D.2.4 — Verify Agent Prompt Quality
Legacy - Run `go test ./internal/agent -v`
- Verify prompt includes architecture context when `ArchSummary` is present
- Verify prompt degrades gracefully when `ArchSummary` is empty

---

## S9D.3 — Generate Architecture Overview Page

**Status:** `todo`

**Description:**
`GenerateIndexPage` 只产出一个三列清单表格。 `dep_graph.json` 和 `nav_plan.json` 中已有足够信息可以生成 Mermaid 依赖关系图和分层说明。在 `RunComposer` 中增加架构总览页生成步骤。

**Acceptance criteria:**
- `architecture.md` generated in the output directory
- Contains purpose description from `ArchSummary`
- Contains Mermaid dependency graph showing module relationships
- VitePress sidebar includes Architecture as the first navigation item
- Existing index page and module docs unchanged

**Files to modify:**
```
internal/composer/renderer.go
internal/composer/vitepress.go
internal/composer/renderer_test.go
internal/composer/vitepress_test.go
```

### Subtasks

#### S9D.3.1 — Add Architecture Page Template Test
Legacy - Test that `generateArchitecturePage` produces valid markdown with Mermaid graph
- Test with empty `ArchSummary` (graceful degradation)
- Test Mermaid graph correctness for various dependency patterns

#### S9D.3.2 — Implement `generateArchitecturePage`
Legacy - Generate `# Architecture Overview` heading and purpose text
- Build Mermaid `graph TD` from `NavPlan` module dependencies
- Write to `architecture.md` in output directory

#### S9D.3.3 — Add Architecture to VitePress Sidebar
Legacy - Add `architecture.md` as first item in VitePress sidebar config
- Ensure correct link and navigation structure

#### S9D.3.4 — Verify Architecture Page Generation
Legacy - Run `go test ./internal/composer -v`
- Re-run `go test ./...` after all Epic 9C changes land

---

## S9D.4 — Final Feature Verification

**Status:** `todo`

**Description:**
Verify the full pipeline produces enhanced documentation with architecture context.

**Acceptance criteria:**
- All focused tests for S9D.1, S9D.2, S9D.3 pass
- `go test ./...` passes with zero failures
- Generated documentation includes architecture page, agent docs reference system context
- No diagnostic errors in changed files
