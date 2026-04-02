# Feature Enhancement — Architecture Summary & Cross-Module Context

**Status:** Draft
**Last updated:** 2026-04-02
**Depends on:** Epic 9B (Go Workspace), Epic 9C (Planner Skeleton)

---

## 1. Problem

当前文档存在割裂感：
1. **Planner** 已经看过全局骨架，是生成架构描述的最佳时机，但产出只包含模块分组（`NavPlan.Modules`），架构上下文被丢弃。
2. **Agent** 对其他非共享模块一无所知，每个模块文档都是孤立的，缺乏系统级视角。
3. **Composer** 只生成三列清单式 index 页，没有架构总览、依赖图、分层说明。

---

## 2. Goal

三步打通架构上下文：
1. Planner 产出 `ArchSummary`（系统目的、分层、数据流、关键模块）
2. Agent 获得跨模块上下文（架构摘要 + 邻居模块信息）
3. Composer 生成架构总览页（含 Mermaid 依赖图）

---

## 3. Design

### 3.1 新类型：`ArchSummary`

**位置：** `pkg/store/artifacts.go`

```go
type ArchSummary struct {
    Purpose     string   `json:"purpose"`                // 1-2 句话描述系统目的
    Layers      []string `json:"layers,omitempty"`        // 架构层次，如 ["API", "Business Logic", "Data"]
    DataFlow    string   `json:"data_flow,omitempty"`     // 端到端数据流描述
    KeyModules  []string `json:"key_modules,omitempty"`   // 3-5 个核心模块 ID
}
```

**NavPlan 扩展：**
```go
type NavPlan struct {
    GeneratedAt         time.Time    `json:"generated_at"`
    Modules             []Module     `json:"modules"`
    ArchitectureSummary *ArchSummary `json:"architecture_summary,omitempty"` // 新增，指针保证向后兼容
}
```

### 3.2 Planner Prompt 扩展

在 `buildPlannerPrompt` 中追加 `architecture_summary` 输出要求：

```
Schema: {
  modules: [...],
  architecture_summary: {
    purpose: "1-2 sentence system description",
    layers: ["layer1", "layer2"],
    data_flow: "end-to-end flow description",
    key_modules: ["module1", "module2"]
  }
}
```

### 3.3 Agent 跨模块上下文

**`AgentInput` 扩展：**

```go
type AgentInput struct {
    Module            store.Module
    FileIndex         store.FileIndex
    SharedContext     store.SharedContext
    Config            *configpkg.Config
    ArchSummary       *store.ArchSummary        // 新增
    NeighborSummaries map[string]string          // 新增：邻居模块 ID -> 简要描述
}
```

**Agent Prompt 新增区块：**

```
## System Architecture
Purpose: {purpose}
Layers: {layers}
Data flow: {data_flow}

## This Module's Role
Upstream: {modules that this depends on}
Downstream: {modules that depend on this}
```

仅当 `ArchSummary` 非空时注入。

### 3.4 Architecture Overview 页

**位置：** `internal/composer/renderer.go`

新函数 `GenerateArchitecturePage(plan *store.NavPlan, graph store.DepGraph) string`：
- 标题 `# Architecture Overview`
- Purpose 描述（来自 `ArchSummary`）
- Layers 列表
- Data flow 描述
- Mermaid `graph TD` 依赖关系图（从 NavPlan 模块依赖构建）

**VitePress sidebar：** 在 Modules 分组前添加 Architecture 链接。

### 3.5 不变的部分

- `Module` 结构体不变
- `SharedSummary` / `SharedContext` 不变
- Preprocessor 不变
- `RunComposer` 签名不变（已接收 `*store.NavPlan`，自动获得新字段）

---

## 4. Acceptance Criteria

1. `store.ArchSummary` 定义正确，`NavPlan.ArchitectureSummary` 可选字段
2. `nav_plan.json` 向后兼容（无 `architecture_summary` 时反序列化正常）
3. Planner prompt 要求 LLM 产出 `architecture_summary`
4. Agent prompt 在 `ArchSummary` 存在时注入架构上下文，不存在时无变化
5. `architecture.md` 包含 purpose、layers、data flow、Mermaid 图
6. VitePress sidebar 包含 Architecture 作为第一个导航项
7. `go test ./...` 零失败

---

## 5. Out of Scope

- Planner LLM 调用策略优化（独立于本 epic）
- 非 Go 语言的 import 解析增强
- 文档质量评分系统
