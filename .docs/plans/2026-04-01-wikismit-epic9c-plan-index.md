# wikismit Epic 9C Plan Index

Use this index instead of executing `.docs/tasks/wikismit-epic9c-planner-skeleton.md` directly.

## Read first

1. `.docs/tasks/wikismit-epic9c-planner-skeleton.md`
2. `.docs/spec/2026-04-01-planner-skeleton-compression.md`

## Execution order

1. `.docs/plans/2026-04-01-wikismit-epic9c-plan-01-build-planner-skeleton.md`
2. `.docs/plans/2026-04-01-wikismit-epic9c-plan-02-integrate-and-verify.md`

## Why this split exists

Epic 9C 为 Planner 添加专用极简 skeleton。工作分两层：

1. **核心实现 (S9C.1):** 新增 `BuildPlannerSkeleton` 函数及其单元测试。这是独立的新增功能，不修改任何现有代码。
2. **集成验证 (S9C.2 + S9C.3):** 将 Planner 调用链从 `BuildFullSkeleton` 切换到 `BuildPlannerSkeleton`，运行全量测试验证无回归。

## Pre-implementation alignment notes

1. `store.Import` 已有 `Internal bool` 和 `ResolvedPath string` 字段，`dep_graph.go` 在 workspace 场景下已正确 populate。`BuildPlannerSkeleton` 只需读取 `entry.Imports`，无需修改 store 或 analyzer。
2. 输出格式：文件路径头行 → 类型名（逗号分隔）→ `->` 内部 import 路径。无类型无 import 的文件只输出路径头保留占位。
3. 截断策略：按文件粒度累积 token，整个文件放入或跳过，不截断单文件内容。这比 `BuildSkeleton` 的行级截断更适合 Planner（Planner 需要完整文件信息来做分组决策）。
4. `prompt.go` 的 skeleton 注入点是 `%s` 占位符，新格式作为纯文本注入，无需修改模板。
5. Agent (`agent/prompt.go`) 和 Preprocessor (`preprocessor.go`) 仍然调用 `BuildSkeleton`，不受影响。

## Commit flow

Recommended commit checkpoints:

1. `BuildPlannerSkeleton` 函数 + 单元测试
2. Planner 调用链切换 + 全量回归测试
