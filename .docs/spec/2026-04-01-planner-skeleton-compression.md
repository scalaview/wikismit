# Planner Skeleton Compression — Technical Spec

**Status:** Draft
**Last updated:** 2026-04-01
**Depends on:** Epic 9B (Go Workspace 支持)

---

## 1. Problem

Go workspace 项目通常包含数百甚至上千个文件。当前 `BuildSkeleton` 对所有消费者（Planner、Agent、Preprocessor）输出相同格式——包含完整的函数签名和类型声明。在默认 `SkeletonMaxTokens: 3000` 的限制下，workspace 项目的 skeleton 被大量截断，Planner 丢失关键的结构信息，无法正确完成模块分组任务。

当前截断策略的问题：
1. **无差别丢弃**：先保留 exported 符号，超出预算后丢弃 unexported。对 Planner 来说，函数签名本身就是不必要的噪音。
2. **丢失依赖信息**：`FileEntry.Imports` 已包含 `Internal` + `ResolvedPath`，但 `BuildSkeleton` 从未读取，Planner 看不到文件间的依赖关系。
3. **消费者需求不同**：Agent 写文档需要函数签名，Preprocessor 需要类型+函数，但 Planner 做模块分组只需要文件结构 + 依赖边。

**量化分析（500 文件 workspace 项目）：**

| 输出格式 | 每文件行数（估算） | 总 token 数 |
|----------|-------------------|------------|
| 当前 skeleton（函数签名+类型） | ~15 行 | ~56,000 |
| 仅 exported 类型名 | ~3 行 | ~15,000 |
| 类型名 + 内部 import 关系 | ~3 行 | ~12,000 |

---

## 2. Goal

为 Planner 提供专用的极简 skeleton，在相同 token 预算下包含更多文件的结构信息和依赖关系，使 Planner 能对大型 workspace 项目正确分组。

---

## 3. Design

### 3.1 新函数：`BuildPlannerSkeleton`

**位置：** `internal/planner/skeleton.go`

**签名：**
```go
func BuildPlannerSkeleton(idx store.FileIndex, maxTokens int) string
```

**输出格式：**
```
// internal/analyzer/analyzer.go
  type Analyzer struct
  type LanguageParser interface
  <- pkg/store, internal/planner

// internal/analyzer/dep_graph.go
  <- pkg/store, internal/planner

// pkg/store/index.go
  type FileIndex, FileEntry, FunctionDecl, TypeDecl, Import, DepGraph, Module, NavPlan
```

**规则：**
- 每个文件一行路径头
- Exported 类型名逗号分隔在同一行（如果有的话）
- 内部 import 用 `->` 表示依赖方向，列出被依赖的包/文件路径（来自 `Import.Internal == true && Import.ResolvedPath != ""`）
- 无类型且无 import 的文件只输出路径头（保留占位，表明文件存在）
- 严格控制在 `maxTokens` 内，超出时按文件粒度截断

### 3.2 消费者路由

| 消费者 | 函数 | 格式 |
|--------|------|------|
| Planner | `BuildPlannerSkeleton` | 类型名 + 内部 import |
| Agent | `BuildSkeleton`（不变） | 函数签名 + 类型 |
| Preprocessor | `BuildSkeleton`（不变） | 函数签名 + 类型 |

### 3.3 配置变更

不新增配置字段。Planner 复用现有的 `SkeletonMaxTokens`，因为极简格式下相同预算能覆盖更多文件。如果用户需要分别控制，可作为后续增强。

### 3.4 不变的部分

- `BuildSkeleton` 和 `BuildFullSkeleton` 的签名和行为完全不变
- Agent 和 Preprocessor 的调用路径不变
- `store.FileIndex` 和 `store.Import` 类型不变
- Token 估算逻辑复用现有的 `estimateTokens*` 系列函数

---

## 4. Acceptance Criteria

1. `BuildPlannerSkeleton` 输出类型名 + 内部 import 关系，不含函数签名
2. 在 `maxTokens` 限制内，比 `BuildFullSkeleton` 覆盖更多文件
3. 输出格式稳定、可解析，Planner prompt 能正确消费
4. `go test ./...` 全部通过
5. 现有 `BuildSkeleton` / `BuildFullSkeleton` 的测试不受影响

---

## 5. Out of Scope

- Agent/Preprocessor skeleton 格式变更
- 新增配置字段
- Skeleton 格式的版本化或序列化
- 非 Go 语言的 import 解析（现有逻辑已处理）
