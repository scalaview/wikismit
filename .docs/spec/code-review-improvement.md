# Wikismit 代码 Review：功能缺口与 OOP 重构建议

> 本文档基于对 `wikismit-master` 全量源码的静态审查，覆盖功能层面的架构缺口和代码实现层面的 OOP 不足，并给出具体的修复建议和优先级排序。

---

## 目录

- [一、功能层面：文档割裂感的根本原因](#一功能层面文档割裂感的根本原因)
- [二、代码实现：OOP 缺失的具体表现](#二代码实现oop-缺失的具体表现)
- [三、其他具体问题](#三其他具体问题)
- [四、优先级总表](#四优先级总表)

---

## 一、功能层面：文档割裂感的根本原因

文档割裂感不是写作风格问题，而是**三个设计缺口**导致的结构性问题。

### 1.1 Planner 只做文件分组，未生成架构摘要

当前 `buildPlannerPrompt` 只要求 LLM 把文件分组，输出的 `nav_plan.json` 中只有 `{id, files, shared, owner, depends_on_shared, referenced_by}`——没有任何系统架构层面的描述。

```go
// internal/planner/prompt.go
func buildPlannerPrompt(skeleton string, threshold int) string {
    return fmt.Sprintf(`You are a software architect. Given this repository skeleton, group the files
into logical documentation modules. ...
Schema: { modules: [{ id, files[], shared, owner, depends_on_shared[], referenced_by[] }] }
```

LLM 在这一步已经"看过全局骨架"，是生成**整体架构描述**的最佳时机。但当前直接丢弃了这一上下文，后续每个 agent 只收到自己模块的 skeleton，自然只能产出孤立文档。

**建议**：扩展 `NavPlan` 结构体，修改 Planner prompt，让其同时产出 `architecture_summary` 字段：

```go
// pkg/store/artifacts.go
type NavPlan struct {
    GeneratedAt         time.Time         `json:"generated_at"`
    ArchitectureSummary ArchSummary       `json:"architecture_summary"`
    Modules             []Module          `json:"modules"`
}

type ArchSummary struct {
    Purpose    string   `json:"purpose"`     // 系统用途
    Layers     []string `json:"layers"`      // 主要分层描述
    DataFlow   string   `json:"data_flow"`   // 核心数据流
    KeyModules []string `json:"key_modules"` // 关键模块角色
}
```

Planner prompt 末尾追加输出要求：

```
Additionally, produce an "architecture_summary" field describing:
- purpose: what this system does in 1-2 sentences
- layers: the major architectural layers (e.g. ["CLI", "Pipeline", "LLM", "Store"])
- data_flow: how data flows through the system end-to-end
- key_modules: the 3-5 most central modules and their roles
```

这段 `ArchSummary` 之后注入到每个 agent prompt 的前缀，agent 就能在整体视角下写文档，不再产出孤立片段。

---

### 1.2 Agent prompt 缺乏跨模块上下文

`BuildAgentPrompt` 的结构是：模块 skeleton → 共享模块引用 → 写作指令。但每个 agent 对其他**非共享**模块一无所知。

```go
// internal/agent/prompt.go
func BuildAgentPrompt(input AgentInput) string {
    skeleton := planner.BuildSkeleton(input.Module.Files, ...)
    sharedBlock := buildSharedModulesBlock(input)
    sections := []string{
        fmt.Sprintf("You are a technical writer documenting the %q module ...", input.Module.ID),
        "## Code skeleton",
        skeleton,
    }
    // sharedBlock 只包含 shared_preprocessor 模块，不包含 agent 模块
```

例如 `pipeline` 的 agent 不知道 `planner`/`agent`/`composer` 的存在关系，所以它的 "Usage Notes" 无法描述自己在流水线中的位置。

**建议**：在 `AgentInput` 中增加架构上下文，并在 prompt 中注入：

```go
// internal/agent/types.go
type AgentInput struct {
    Module              store.Module
    FileIndex           store.FileIndex
    SharedContext       store.SharedContext
    Config              *configpkg.Config
    ArchSummary         store.ArchSummary  // ← 新增
    NeighborSummaries   map[string]string  // ← 新增：上下游模块的一句话摘要
}
```

prompt 头部追加：

```
## System Architecture Context
{arch_summary.purpose}
Layers: {arch_summary.layers}
Data flow: {arch_summary.data_flow}

## This module's role
Upstream: {neighbor_summaries.upstream}
Downstream: {neighbor_summaries.downstream}
```

---

### 1.3 Composer 未生成架构总览页

`GenerateIndexPage` 只产出一个三列清单表格：

```go
// internal/composer/renderer.go
func GenerateIndexPage(plan *store.NavPlan, graph store.DepGraph) string {
    ...
    builder.WriteString("| Module | Type | Used By |\n")
    builder.WriteString("| --- | --- | --- |\n")
    ...
}
```

`dep_graph.json` 和 `nav_plan.json` 中已有足够信息可以生成 Mermaid 依赖关系图和分层说明，但完全未使用。VitePress sidebar 里也只有 Modules 和 Shared，没有任何高层导航入口。

**建议**：在 `RunComposer` 中增加 `generateArchitecturePage` 步骤：

```go
// internal/composer/renderer.go
func (c *Composer) generateArchitecturePage() error {
    var b strings.Builder
    b.WriteString("# Architecture Overview\n\n")
    b.WriteString(c.plan.ArchitectureSummary.Purpose + "\n\n")

    // 生成 Mermaid 依赖图
    b.WriteString("## Module Dependency Graph\n\n")
    b.WriteString("```mermaid\ngraph TD\n")
    for _, module := range c.plan.Modules {
        for _, dep := range module.DependsOnShared {
            b.WriteString(fmt.Sprintf("    %s --> %s\n", module.ID, dep))
        }
    }
    b.WriteString("```\n\n")

    // 写入 architecture.md
    path := filepath.Join(c.config.OutputDir, "architecture.md")
    return os.WriteFile(path, []byte(b.String()), 0o644)
}
```

同时在 VitePress sidebar 配置中将其作为首个导航项。

---

## 二、代码实现：OOP 缺失的具体表现

### 2.1 ⚠️ Critical：`RunPlanner` 中存在全局 logger 并发写

这是最严重的并发 bug，现有 review spec 未提及。

```go
// internal/planner/planner.go
func RunPlanner(ctx context.Context, ...) (*store.NavPlan, error) {
    plannerLogger := logger        // ① 读全局变量
    if plannerLogger == nil {
        plannerLogger = logpkg.New(cfg.Verbose)
        logger = plannerLogger     // ② 写全局变量！
        defer func() {
            logger = nil           // ③ defer 重置全局变量！
        }()
    }
```

`RunPlanner` 虽然在 `RunFullGenerate` 中顺序调用，但 `incremental.go` 存在并发场景，一旦多个 goroutine 同时触发，②③ 两步会产生**数据竞争**（data race），`go test -race` 会直接报错。根本原因是把 logger 作为包级全局变量，而非结构体字段。

同样的问题存在于：
- `internal/planner/skeleton.go` — `var logger logpkg.Logger`
- `internal/preprocessor/shared_context.go` — `var sharedLogger logpkg.Logger`

**修复**：将三处包级 logger 全部移除，改用 `Planner` 结构体注入：

```go
// internal/planner/planner.go
type Planner struct {
    client llm.Client
    config *configpkg.Config
    logger logpkg.Logger
}

func NewPlanner(client llm.Client, cfg *configpkg.Config, logger logpkg.Logger) *Planner {
    if logger == nil {
        logger = logpkg.New(cfg.Verbose)
    }
    return &Planner{client: client, config: cfg, logger: logger}
}

func (p *Planner) Plan(ctx context.Context, idx store.FileIndex, graph store.DepGraph) (*store.NavPlan, error) {
    // 原 RunPlanner 逻辑，全部使用 p.logger，无任何全局状态
}
```

---

### 2.2 ⚠️ Critical：`GenerateIndexPage` 中依赖图排序逻辑永远失效

这是一个**逻辑 bug**，现有 review spec 未提及。

```go
// internal/composer/renderer.go
func GenerateIndexPage(plan *store.NavPlan, graph store.DepGraph) string {
    modules := append([]store.Module(nil), plan.Modules...)
    sort.Slice(modules, func(i int, j int) bool {
        leftDepth := dependencyDepth(modules[i].ID, graph, map[string]bool{})
        // ...
    })
}

func dependencyDepth(moduleID string, graph store.DepGraph, seen map[string]bool) int {
    deps := graph[moduleID]  // graph 的 key 是文件路径，不是模块 ID！
    // ...
}
```

`graph` 是 `store.DepGraph`，即 `map[filePath][]filePath`，key 是 `"internal/pipeline/incremental.go"` 这样的文件路径，而 `moduleID` 是 `"pipeline"` 这样的模块名。两者永远不会匹配，`deps` 始终为空，所有模块深度均为 0，排序实际上退化为按原始顺序。

**修复**：`dependencyDepth` 应接收模块级别的依赖图，而非文件级 `DepGraph`：

```go
// 从 NavPlan 构建模块依赖图
func buildModuleGraph(plan *store.NavPlan) map[string][]string {
    graph := make(map[string][]string, len(plan.Modules))
    for _, module := range plan.Modules {
        deps := append([]string(nil), module.DependsOnShared...)
        graph[module.ID] = deps
    }
    return graph
}

func GenerateIndexPage(plan *store.NavPlan, _ store.DepGraph) string {
    moduleGraph := buildModuleGraph(plan)
    sort.Slice(modules, func(i, j int) bool {
        leftDepth := dependencyDepth(modules[i].ID, moduleGraph, map[string]bool{})
        rightDepth := dependencyDepth(modules[j].ID, moduleGraph, map[string]bool{})
        // ...
    })
}
```

---

### 2.3 ⚠️ Critical：`incremental.go` 七个全局函数变量，并发不安全

```go
// internal/pipeline/incremental.go
var runGenerateFallback  = RunFullGenerate
var getChangedFiles      = gitdiff.GetChangedFiles
var computeAffected      = analyzer.ComputeAffected
var runPreprocessorFor   = preprocessor.RunPreprocessorFor
var runAgentFor          = agent.RunFor
var runComposer          = composer.RunComposer
var reanalyzeChangedFunc = reanalyzeChanged
```

这是典型的"全局变量 mock"测试模式。在并发测试下会相互污染，且一旦有两个并发的 `RunIncremental` 调用，共享这些变量就会产生竞争。

**修复**：定义 `Pipeline` 结构体，依赖以接口注入：

```go
// internal/pipeline/pipeline.go
type Pipeline struct {
    config       *configpkg.Config
    client       llm.Client
    logger       logpkg.Logger
    analyzer     AnalyzerIface
    planner      PlannerIface
    preprocessor PreprocessorIface
    agentRunner  AgentRunnerIface
    composer     ComposerIface
}

func NewPipeline(cfg *configpkg.Config, client llm.Client, logger logpkg.Logger) *Pipeline {
    return &Pipeline{
        config:       cfg,
        client:       client,
        logger:       logger,
        analyzer:     analyzer.NewAnalyzer(cfg.Analysis),
        planner:      planner.NewPlanner(client, cfg, logger),
        preprocessor: preprocessor.NewPreprocessor(cfg, client, logger),
        agentRunner:  agent.NewOrchestrator(client, cfg.ArtifactsDir, cfg.Agent.Concurrency, logger),
        composer:     composer.NewComposer(cfg, logger),
    }
}

func (p *Pipeline) RunFull(ctx context.Context) error { ... }
func (p *Pipeline) RunIncremental(ctx context.Context, opts IncrementalOptions) error { ... }
```

---

### 2.4 `DepGraphBuilder` 是空壳结构体，与独立函数并存

```go
// internal/analyzer/dep_graph.go

// 风格一：独立函数（被 phase1.go、affected.go 直接调用）
func BuildDepGraph(idx store.FileIndex) store.DepGraph { return buildDepGraph(idx) }

// 风格二：结构体（同文件，但无额外状态或行为）
type DepGraphBuilder struct { idx store.FileIndex }
func NewDepGraphBuilder(idx store.FileIndex) *DepGraphBuilder { ... }
func (b *DepGraphBuilder) Build() store.DepGraph { return buildDepGraph(b.idx) }
```

`DepGraphBuilder` 没有额外状态，只是对 `buildDepGraph` 的无意义封装，却与独立函数并存，让调用方不知道该用哪个。`ResolveImportPaths`、`RunPhase1`、`reanalyzeChanged` 各自使用不同的调用方式。

**建议**：删除 `DepGraphBuilder`，统一使用 `Analyzer` 结构体的方法或独立函数，保持一种风格。

---

### 2.5 `ImpactAnalyzer` 只有外壳，内部逻辑仍是过程式

```go
// internal/analyzer/affected.go

// 私有函数（实际业务逻辑所在）
func owningModules(changedFiles []gitdiff.FileChange, plan *store.NavPlan) []string
func buildReverseGraph(graph store.DepGraph) store.DepGraph
func computeAffected(changedFiles []gitdiff.FileChange, ...) []store.Module

// 公开结构体（后加的，没有真正使用私有方法）
type ImpactAnalyzer struct { plan *store.NavPlan; graph store.DepGraph }
func (a *ImpactAnalyzer) Analyze(changedFiles []gitdiff.FileChange) []store.Module {
    return computeAffected(changedFiles, a.plan, a.graph)  // 只是调用私有函数
}
```

`ImpactAnalyzer` 是后期加的结构体，但 `owningModules` 和 `buildReverseGraph` 没有移为其方法，结构体只是一个薄包装。`computeAffected` 内部也重复了 `owningModules` 的部分逻辑（`fileOwners` 的构建）。

**建议**：将私有函数全部移为 `ImpactAnalyzer` 的私有方法，消除重复逻辑：

```go
type ImpactAnalyzer struct {
    plan  *store.NavPlan
    graph store.DepGraph
}

func (a *ImpactAnalyzer) Analyze(changedFiles []gitdiff.FileChange) []store.Module {
    ownerIDs := a.owningModules(changedFiles)
    // ... 内部完整实现，不依赖任何包级函数
}

func (a *ImpactAnalyzer) owningModules(changedFiles []gitdiff.FileChange) []string { ... }
func (a *ImpactAnalyzer) buildReverseGraph() store.DepGraph { ... }
```

---

### 2.6 `for true` 无续写次数上限，存在无限循环风险

```go
// internal/llm/client.go
for true {   // 应为 for {}
    resp, err := c.complete(requestCtx, &req, preoutput)
    if err != nil {
        return "", err
    }
    if resp.FinishReason == openai.FinishReasonLength {
        builder.WriteString(resp.Message.Content)
        preoutput = builder.String()
        continue   // ← 无次数限制
    }
    builder.WriteString(resp.Message.Content)
    break
}
```

`for true` 不是 Go 惯用法（应使用 `for {}`），但更严重的是：没有**最大续写次数**保护。若模型反复返回 `FinishReasonLength`，会无限续写直到 context 超时并消耗大量 token 配额。

**修复**：

```go
const maxContinuations = 5

for i := 0; i < maxContinuations; i++ {
    resp, err := c.complete(requestCtx, &req, preoutput)
    if err != nil {
        return "", err
    }
    builder.WriteString(resp.Message.Content)
    if resp.FinishReason != openai.FinishReasonLength {
        break
    }
    if i == maxContinuations-1 {
        c.logger.Warn("max continuations reached, truncating response")
        break
    }
    preoutput = builder.String()
}
```

---

### 2.7 `topoSort` 是死代码，Tarjan SCC 缺失必要注释

```go
// internal/preprocessor/preprocessor.go

// topoSort — 只有测试调用，没有任何生产代码调用
func topoSort(graph map[string][]string) ([]string, error) { ... }

// topoSortComponents — 实际被 runPreprocessor 调用的是这个
func topoSortComponents(graph map[string][]string) [][]string { ... }  // ~100 行 Tarjan SCC
```

`topoSort` 是早期遗留代码，应清理。更重要的是 Tarjan SCC 实现近 100 行，隐藏在 preprocessor 包中，没有任何注释说明**为什么**需要 SCC 而不是简单拓扑排序（原因：共享模块之间可能存在循环依赖，SCC 先找到强连通分量再做拓扑排序，可以正确处理循环）。

**建议**：
1. 删除 `topoSort`（或保留但标注 `// only used in tests`）
2. 将 `topoSortComponents` 提取到独立文件 `preprocessor/topo.go`，并加注释说明算法选择的理由
3. 如果后续 `Pipeline` 结构体化，可进一步提取到 `internal/graph` 包

---

## 三、其他具体问题

### 3.1 `resolveClientWithFallback` 语义不清晰

```go
// cmd/wikismit/helpers.go
func resolveClientWithFallback(primaryFactory, fallbackFactory func() llm.Client, cfg *configpkg.Config) (llm.Client, error) {
    if client := primaryFactory(); client != nil {
        return client, nil
    }
    return resolveClient(fallbackFactory, cfg)
    // resolveClient 内部：若 fallbackFactory() 也返回 nil，则调用 llm.NewClient(cfg)
}
```

函数名暗示"三层降级"（primary → fallback → config），但实际是"两层降级"（primary → NewClient(cfg)），`fallbackFactory` 在这里只是另一个 primary 检查，命名具有误导性。调用方阅读时容易误以为 fallback 会使用不同的 LLM 配置。

**建议**：重命名为 `resolveClientWithOverride` 并补充注释，或简化为两个参数。

---

### 3.2 `applyDefaults` 与 `defaultConfig` 职责混淆

```go
// internal/config/config.go
func LoadConfig(path string) (*Config, error) {
    cfg := defaultConfig()            // ① 用默认值初始化
    yaml.Unmarshal(data, &cfg)        // ② YAML 覆盖
    applyDefaults(&cfg)               // ③ 再次补充零值
    ...
}
```

三步流程初看合理，但 `applyDefaults` 中混入了业务逻辑：

```go
if cfg.LLM.PreprocessorModel == "" {
    cfg.LLM.PreprocessorModel = cfg.LLM.PlannerModel  // ← 这是业务规则，不是默认值
}
```

"PreprocessorModel 未配置时 fallback 到 PlannerModel"是业务决策，不是字段默认值，不应放在 `applyDefaults`。建议将业务规则提取到独立的 `normalizeConfig` 函数中，与默认值填充分开。

---

### 3.3 `ValidateDocs` 中断链检测的行号始终为 0

```go
// internal/composer/validator.go
report.BrokenLinks = append(report.BrokenLinks, store.BrokenLink{
    SourceFile: path,
    LinkText:   linkText,
    LinkTarget: target,
    Line:       0,  // ← 始终为 0，从未计算
})
```

`BrokenLink` 结构体有 `Line` 字段，但从未赋值，导致输出报告无法定位到具体行。`FindAllStringSubmatchIndex` 返回的是字节偏移，可以用换行符计数转换为行号。

---

## 四、优先级总表

| 优先级 | 类型 | 问题描述 | 位置 |
|--------|------|----------|------|
| **Critical** | Bug | `RunPlanner` 全局 logger 并发写，data race | `planner/planner.go` |
| **Critical** | Bug | `dependencyDepth` 用文件路径图做模块排序，逻辑永远失效 | `composer/renderer.go` |
| **Critical** | 设计 | 七个全局函数变量用于 mock，并发不安全 | `pipeline/incremental.go` |
| **High** | 功能 | Planner 不产出架构摘要，是文档割裂感的根本原因 | `planner/prompt.go` + `planner.go` |
| **High** | 功能 | Agent prompt 缺乏跨模块上下文，无法描述模块在系统中的角色 | `agent/prompt.go` |
| **High** | 功能 | Composer 无架构总览页，无 Mermaid 依赖图 | `composer/renderer.go` |
| **High** | Bug | `for true` 无续写次数上限，存在无限 token 消耗风险 | `llm/client.go` |
| **Medium** | OOP | `DepGraphBuilder` 是空壳，与独立函数并存，双重 API | `analyzer/dep_graph.go` |
| **Medium** | OOP | `ImpactAnalyzer` 只有外壳，内部逻辑仍是过程式堆砌 | `analyzer/affected.go` |
| **Medium** | 代码质量 | `topoSort` 是死代码；Tarjan SCC 实现无注释 | `preprocessor/preprocessor.go` |
| **Low** | 代码质量 | `resolveClientWithFallback` 命名有误导性 | `cmd/wikismit/helpers.go` |
| **Low** | 代码质量 | `applyDefaults` 混入业务规则，与默认值填充职责混淆 | `config/config.go` |
| **Low** | 代码质量 | `ValidateDocs` 的 `BrokenLink.Line` 始终为 0 | `composer/validator.go` |

---

## 附：最小可行改动路线

如果要以最小改动优先解决最高价值的问题，建议按以下顺序执行：

1. **修复 `dependencyDepth` 使用错误的图**（10 行改动，立刻让 index 页排序生效）
2. **扩展 `NavPlan` + Planner prompt 输出 `ArchitectureSummary`**（直接解决文档割裂感，改动集中在 3 处：`store/artifacts.go`、`planner/prompt.go`、`agent/prompt.go`）
3. **用 `Planner` 结构体替换包级 logger 全局变量**（消除 data race，改动范围可控）
4. **用 `Pipeline` 结构体替换七个全局函数变量**（改动量最大，但可逐步替换，测试通过后删除旧函数变量）