# Plan 02 — 接入 Planner 调用链 + 集成验证

**Epic ref:** S9C.2 + S9C.3
**Depends on:** Plan 01

---

## Goal

将 Planner 的 skeleton 调用从 `BuildFullSkeleton` 切换到 `BuildPlannerSkeleton`，然后运行全量测试验证无回归。

---

## Implementation steps

### Step 1: 替换 Planner 调用

**File:** `internal/planner/planner.go`

当前代码（约第 27-28 行）：
```go
skeleton := BuildFullSkeleton(idx, cfg.Agent.SkeletonMaxTokens)
prompt := buildPlannerPrompt(skeleton, cfg.Analysis.SharedModuleThreshold)
```

修改为：
```go
skeleton := BuildPlannerSkeleton(idx, cfg.Agent.SkeletonMaxTokens)
prompt := buildPlannerPrompt(skeleton, cfg.Analysis.SharedModuleThreshold)
```

仅此一行变更。`buildPlannerPrompt` 的 `%s` 占位符接受任意格式文本，无需修改模板。

---

### Step 2: 确认 Agent 和 Preprocessor 不受影响

验证以下文件仍使用 `BuildSkeleton`（非 `BuildPlannerSkeleton`）：

- `internal/agent/prompt.go` — `planner.BuildSkeleton(input.Module.Files, input.FileIndex, ...)`
- `internal/preprocessor/preprocessor.go` — `planner.BuildSkeleton(files, idx, ...)`

如果上述文件仍调用 `BuildSkeleton`，则无需改动。

---

### Step 3: 全量测试

```bash
go test ./... -v
```

验证：
- `./internal/planner` — 所有测试通过（包括新增的 `BuildPlannerSkeleton` 测试和原有的 `BuildSkeleton` 测试）
- `./internal/agent` — 测试通过（仍使用 `BuildSkeleton`）
- `./internal/preprocessor` — 测试通过（仍使用 `BuildSkeleton`）
- `./internal/analyzer` — 测试通过（workspace 支持不受影响）
- 全量零失败

---

## Verification

```bash
go test ./... -count=1
```

零失败即完成。
