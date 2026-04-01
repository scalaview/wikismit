# wikismit — Epic 9C: Planner Skeleton 压缩

**Status:** `todo`
**Depends on:** Epic 9B (Go Workspace 支持)
**Goal:** 为 Planner 提供专用的极简 skeleton 格式（类型名 + 内部 import），在相同 token 预算下覆盖更多文件，解决大型 workspace 项目 skeleton 被截断导致模块分组失败的问题。
**Spec refs:** `.docs/spec/2026-04-01-planner-skeleton-compression.md`

---

## S9C.1 — 实现 BuildPlannerSkeleton

**Status:** `todo`

**Description:**
在 `internal/planner/skeleton.go` 中新增 `BuildPlannerSkeleton(idx store.FileIndex, maxTokens int) string` 函数。遍历 FileIndex 中所有文件，输出文件路径、exported 类型名、内部 import 关系（`Import.Internal == true` 的 `ResolvedPath`），不含函数签名。按文件粒度截断以保持在 `maxTokens` 内。

**Acceptance criteria:**
- 输出格式：文件路径头 + 类型名行 + import 行（`->` 前缀）
- 不包含任何函数签名
- 在相同 `maxTokens` 下比 `BuildFullSkeleton` 覆盖更多文件
- 无类型无 import 的文件只输出路径头（保留占位）
- 超出预算时按文件粒度截断，不截断单文件内容

**Files to modify:**
```
internal/planner/skeleton.go
```

### Subtasks

#### S9C.1.1 — 实现 BuildPlannerSkeleton 核心逻辑
- 遍历 `idx` 中所有文件，按字母排序
- 对每个文件：输出路径头行、exported 类型名（逗号分隔）、内部 import 的 `ResolvedPath`
- 按文件粒度累积 token，超出 `maxTokens` 时停止添加新文件
- 复用现有 `estimateTokens*` 系列函数

#### S9C.1.2 — 为 BuildPlannerSkeleton 编写单元测试
- 测试基本输出：包含文件路径、类型名、import 关系
- 测试不包含函数签名
- 测试 token 截断行为：超出预算时按文件粒度截断
- 测试空文件（无类型无 import）只输出路径头
- 测试纯外部 import 不出现在 `->` 行中

---

## S9C.2 — 接入 Planner 调用链

**Status:** `todo`

**Description:**
修改 `internal/planner/planner.go`，将 `BuildFullSkeleton` 替换为 `BuildPlannerSkeleton`。确保 Planner prompt 模板能正确消费新格式。

**Acceptance criteria:**
- `planner.go` 调用 `BuildPlannerSkeleton` 而非 `BuildFullSkeleton`
- `prompt.go` 模板不变（skeleton 作为纯文本注入，格式无关）
- 现有 `BuildSkeleton` 和 `BuildFullSkeleton` 不受影响
- Agent 和 Preprocessor 仍然使用 `BuildSkeleton`

**Files to modify:**
```
internal/planner/planner.go
```

### Subtasks

#### S9C.2.1 — 替换 Planner 调用
- 将 `planner.go` 中的 `BuildFullSkeleton(idx, cfg.Agent.SkeletonMaxTokens)` 替换为 `BuildPlannerSkeleton(idx, cfg.Agent.SkeletonMaxTokens)`
- 确认 `prompt.go` 的 `%s` 占位符能容纳新格式

---

## S9C.3 — 集成验证

**Status:** `todo`

**Description:**
验证全量测试通过，包括新增测试和现有测试。

**Acceptance criteria:**
- `go test ./...` 零失败
- 现有 `skeleton_test.go` 中的 `BuildSkeleton` / `BuildFullSkeleton` 测试全部通过
- 新增 `BuildPlannerSkeleton` 测试全部通过
- 无 lint 或诊断错误

**Files to modify:**
```
(none — verification only)
```
