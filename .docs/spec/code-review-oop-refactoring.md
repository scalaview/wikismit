# Wikismit 代码质量审查与面向对象重构规范

## 概述

本文档对 wikismit 项目进行全面代码审查，重点关注：
1. 逻辑正确性
2. 安全问题
3. 面向过程 vs 面向对象设计分析
4. 改进建议与重构规范

---

## 一、整体架构评估

### 当前状态
项目采用分层架构：
- `cmd/wikismit/` - CLI 入口层
- `internal/` - 核心业务逻辑
- `pkg/` - 公共包

### 主要问题
项目整体偏向**面向过程设计**，存在以下模式：
- 大量独立函数而非方法
- 数据结构与行为分离
- 缺少领域模型抽象
- 全局变量和包级别状态

---

## 二、逻辑正确性问题

### Critical (严重)

#### 2.1 无限循环风险
**文件**: `internal/llm/client.go:73`
```go
for true {  // 应使用 for { 或明确的循环条件
    resp, err := c.complete(requestCtx, &req, preoutput)
    // ...
}
```
**问题**: `for true` 虽然可工作，但不符合 Go 惯例，且没有明确的退出条件检查。

**修复建议**:
```go
for {
    resp, err := c.complete(requestCtx, &req, preoutput)
    if err != nil {
        return "", err
    }
    if resp.FinishReason != openai.FinishReasonLength {
        builder.WriteString(resp.Message.Content)
        break
    }
    builder.WriteString(resp.Message.Content)
    preoutput = builder.String()
}
```

#### 2.2 全局变量导致的并发问题
**文件**: `internal/planner/planner.go:12`
```go
var logger logpkg.Logger = logpkg.New(false)
```
**文件**: `internal/preprocessor/shared_context.go:11`
```go
var sharedLogger logpkg.Logger = logpkg.New(false)
```
**文件**: `internal/planner/skeleton.go:12`
```go
var logger logpkg.Logger = logpkg.New(false)
```

**问题**: 包级别全局变量在并发场景下可能导致数据竞争和不一致状态。

**修复建议**: 将 logger 作为依赖注入到结构体中。

### Important (重要)

#### 2.3 错误处理不一致
**文件**: `internal/llm/client.go:147`
```go
return nil, normalizedErr  // 返回 nil 和 error
```
**问题**: 返回 `nil, error` 模式在某些情况下可能导致调用方混淆。

**修复建议**: 统一错误处理模式，使用明确的错误返回。

#### 2.4 潜在的资源泄漏
**文件**: `internal/llm/retry.go:56-64`
```go
timer := time.NewTimer(wait)
select {
case <-ctx.Done():
    if !timer.Stop() {
        <-timer.C
    }
    return "", ctx.Err()
case <-timer.C:
}
```
**问题**: timer.C channel 在 `timer.Stop()` 返回 false 时需要被排空，当前实现正确，但可以更简洁。

#### 2.5 边界条件未处理
**文件**: `internal/analyzer/lang/golang.go:194`
```go
func isExported(name string) bool {
    if name == "" {
        return false
    }
    return unicode.IsUpper([]rune(name)[0])
}
```
**问题**: 已有空字符串检查，但 `[]rune(name)[0]` 在极端情况下可能有问题。

---

## 三、安全问题

### Critical (严重)

#### 3.1 路径遍历风险
**文件**: `cmd/wikismit/build.go:50-58`
```go
nodeModulesPath := filepath.Join(cfg.OutputDir, "node_modules")
packageJSONPath := filepath.Join(cfg.OutputDir, "package.json")
```
**问题**: `cfg.OutputDir` 来自配置文件，未经验证直接用于文件操作，可能导致路径遍历攻击。

**修复建议**:
```go
func validatePath(basePath, targetPath string) error {
    absBase, err := filepath.Abs(basePath)
    if err != nil {
        return err
    }
    absTarget, err := filepath.Abs(targetPath)
    if err != nil {
        return err
    }
    if !strings.HasPrefix(absTarget, absBase) {
        return errors.New("path traversal detected")
    }
    return nil
}
```

#### 3.2 命令注入风险
**文件**: `cmd/wikismit/build.go:15-21`
```go
var runCommand = func(dir string, name string, args ...string) error {
    cmd := exec.Command(name, args...)
    cmd.Dir = dir
    // ...
}
```
**问题**: 虽然 `name` 来自硬编码（"npm", "npx"），但 `dir` 来自配置，需要验证。

### Important (重要)

#### 3.3 敏感信息泄露
**文件**: `internal/llm/client.go:100-108`
```go
c.logger.Debug("starting chat completion request",
    "model", model,
    "base_url", c.baseURL,
    "user_prompt_chars", len(req.UserMsg),
)
```
**问题**: 日志中可能包含敏感信息（虽然当前只记录字符数，但需要确保未来不会意外记录完整 prompt）。

#### 3.4 YAML 解析安全
**文件**: `internal/config/config.go:158`
```go
if err := yaml.Unmarshal(data, &cfg); err != nil {
    return nil, fmt.Errorf("parse config %q: %w", path, err)
}
```
**问题**: 使用 `gopkg.in/yaml.v3` 是安全的，但应确保配置文件权限正确。

---

## 四、面向过程 vs 面向对象分析

### 4.1 需要重构的核心领域

#### A. Agent 包 (`internal/agent/`)

**当前状态**:
```go
// 独立函数
func Run(ctx context.Context, modules []store.Module, input AgentInput, ...) error
func runScheduler(ctx context.Context, modules []store.Module, ...) error
func runAgent(ctx context.Context, module store.Module, input AgentInput, ...) ModuleDoc
```

**重构建议**:
```go
// AgentOrchestrator 协调多个 Agent 的执行
type AgentOrchestrator struct {
    client       llm.Client
    artifactsDir string
    concurrency  int
    logger       logpkg.Logger
}

func NewAgentOrchestrator(client llm.Client, artifactsDir string, concurrency int, logger logpkg.Logger) *AgentOrchestrator {
    return &AgentOrchestrator{
        client:       client,
        artifactsDir: artifactsDir,
        concurrency:  concurrency,
        logger:       logger,
    }
}

func (o *AgentOrchestrator) Run(ctx context.Context, modules []store.Module, input AgentInput) error {
    // 实现细节
}

func (o *AgentOrchestrator) RunFor(ctx context.Context, modules []store.Module, input AgentInput) error {
    filtered := o.filterModules(modules, "agent")
    if len(filtered) == 0 {
        return nil
    }
    return o.Run(ctx, filtered, input)
}

// Agent 单个模块的文档生成代理
type Agent struct {
    module store.Module
    input  AgentInput
    client llm.Client
    logger logpkg.Logger
}

func NewAgent(module store.Module, input AgentInput, client llm.Client, logger logpkg.Logger) *Agent {
    return &Agent{module: module, input: input, client: client, logger: logger}
}

func (a *Agent) Execute(ctx context.Context) ModuleDoc {
    // 实现
}
```

**改进原因**:
- 封装执行逻辑和依赖
- 便于测试（可 mock）
- 支持不同策略实现
- 状态管理更清晰

#### B. LLM 包 (`internal/llm/`)

**当前状态**:
```go
// client.go
type openAIClient struct { ... }
func NewClient(cfg configpkg.LLMConfig) (Client, error)

// retry.go - 独立结构但设计良好
type retryingClient struct { ... }
```

**重构建议**:
```go
// ClientBuilder 构建 LLM 客户端
type ClientBuilder struct {
    config     configpkg.LLMConfig
    logger     logpkg.Logger
    maxRetries int
}

func NewClientBuilder(config configpkg.LLMConfig) *ClientBuilder {
    return &ClientBuilder{config: config}
}

func (b *ClientBuilder) WithLogger(logger logpkg.Logger) *ClientBuilder {
    b.logger = logger
    return b
}

func (b *ClientBuilder) WithMaxRetries(n int) *ClientBuilder {
    b.maxRetries = n
    return b
}

func (b *ClientBuilder) Build() (Client, error) {
    if b.logger == nil {
        b.logger = logpkg.New(false)
    }

    var client Client
    client, err := newOpenAIClient(b.config, b.logger)
    if err != nil {
        return nil, err
    }

    if b.maxRetries > 0 {
        client = NewRetryingClient(client, b.maxRetries, b.logger)
    }

    return client, nil
}

// CompletionRequest 添加验证方法
func (r *CompletionRequest) Validate() error {
    if r.MaxTokens <= 0 {
        return errors.New("max tokens must be positive")
    }
    if r.UserMsg == "" {
        return errors.New("user message is required")
    }
    return nil
}
```

#### C. Analyzer 包 (`internal/analyzer/`)

**当前状态**:
```go
// analyzer.go - 已有结构体但方法不完整
type Analyzer struct { ... }

// dep_graph.go - 独立函数
func BuildDepGraph(idx store.FileIndex) store.DepGraph
func ResolveImportPaths(repoPath string, ...) (store.FileIndex, error)

// affected.go - 独立函数
func ComputeAffected(changedFiles []gitdiff.FileChange, ...) []store.Module
```

**重构建议**:
```go
// Analyzer 扩展现有结构
type Analyzer struct {
    registry        map[string]LanguageParser
    excludePatterns []string
    modulePath      string
    skippedFiles    int
    logger          logpkg.Logger  // 添加日志
}

// DepGraphBuilder 依赖图构建器
type DepGraphBuilder struct {
    fileIndex store.FileIndex
    logger    logpkg.Logger
}

func NewDepGraphBuilder(idx store.FileIndex, logger logpkg.Logger) *DepGraphBuilder {
    return &DepGraphBuilder{fileIndex: idx, logger: logger}
}

func (b *DepGraphBuilder) Build() store.DepGraph {
    // 实现
}

// ImpactAnalyzer 影响分析器
type ImpactAnalyzer struct {
    plan      *store.NavPlan
    graph     store.DepGraph
    logger    logpkg.Logger
}

func NewImpactAnalyzer(plan *store.NavPlan, graph store.DepGraph, logger logpkg.Logger) *ImpactAnalyzer {
    return &ImpactAnalyzer{plan: plan, graph: graph, logger: logger}
}

func (a *ImpactAnalyzer) ComputeAffected(changedFiles []gitdiff.FileChange) []store.Module {
    // 实现
}

func (a *ImpactAnalyzer) OwningModules(changedFiles []gitdiff.FileChange) []string {
    // 实现
}
```

#### D. Composer 包 (`internal/composer/`)

**当前状态**:
```go
// renderer.go - 全部独立函数
func GenerateTOC(content string) string
func CopyModuleDocs(artifactsDir string, ...) error
func GenerateIndexPage(plan *store.NavPlan, ...) string
func RunComposer(cfg *configpkg.Config, ...) error
```

**重构建议**:
```go
// Composer 文档组合器
type Composer struct {
    config     *configpkg.Config
    plan       *store.NavPlan
    fileIndex  store.FileIndex
    depGraph   store.DepGraph
    logger     logpkg.Logger
}

func NewComposer(cfg *configpkg.Config, plan *store.NavPlan, idx store.FileIndex, graph store.DepGraph, logger logpkg.Logger) *Composer {
    return &Composer{
        config:    cfg,
        plan:      plan,
        fileIndex: idx,
        depGraph:  graph,
        logger:    logger,
    }
}

func (c *Composer) Run() error {
    symbolMap := c.buildSymbolMap()
    if err := c.copyModuleDocs(symbolMap); err != nil {
        return err
    }
    if err := c.generateIndexPage(); err != nil {
        return err
    }
    return c.generateVitePressAssets()
}

func (c *Composer) copyModuleDocs(symbolMap map[string]string) error {
    // 实现
}

func (c *Composer) generateIndexPage() error {
    // 实现
}

// TOCGenerator 目录生成器
type TOCGenerator struct{}

func (g *TOCGenerator) Generate(content string) string {
    // 实现
}

// CitationInjector 引用注入器
type CitationInjector struct {
    symbolMap map[string]string
}

func NewCitationInjector(symbolMap map[string]string) *CitationInjector {
    return &CitationInjector{symbolMap: symbolMap}
}

func (i *CitationInjector) Inject(content string) string {
    // 实现
}
```

#### E. Preprocessor 包 (`internal/preprocessor/`)

**当前状态**:
```go
// preprocessor.go - 独立函数
func RunPreprocessor(ctx context.Context, ...) (store.SharedContext, error)
func RunPreprocessorFor(ctx context.Context, ...) (store.SharedContext, error)

// shared_context.go - 全局变量
var sharedLogger logpkg.Logger = logpkg.New(false)
func groundSharedSummaryRefs(...) store.SharedSummary
```

**重构建议**:
```go
// Preprocessor 预处理器
type Preprocessor struct {
    config  *configpkg.Config
    client  llm.Client
    logger  logpkg.Logger
}

func NewPreprocessor(cfg *configpkg.Config, client llm.Client, logger logpkg.Logger) *Preprocessor {
    return &Preprocessor{config: cfg, client: client, logger: logger}
}

func (p *Preprocessor) Run(ctx context.Context, plan *store.NavPlan, idx store.FileIndex, graph store.DepGraph) (store.SharedContext, error) {
    return p.run(ctx, nil, plan, idx, graph)
}

func (p *Preprocessor) RunFor(ctx context.Context, affected []store.Module, plan *store.NavPlan, idx store.FileIndex, graph store.DepGraph) (store.SharedContext, error) {
    affectedSet := p.buildAffectedSet(affected)
    return p.run(ctx, affectedSet, plan, idx, graph)
}

func (p *Preprocessor) run(ctx context.Context, affectedSet map[string]bool, ...) (store.SharedContext, error) {
    // 实现
}

// TopologicalSorter 拓扑排序器
type TopologicalSorter struct {
    graph map[string][]string
}

func NewTopologicalSorter(graph map[string][]string) *TopologicalSorter {
    return &TopologicalSorter{graph: graph}
}

func (s *TopologicalSorter) Sort() ([]string, error) {
    // 实现
}
```

#### F. Pipeline 包 (`internal/pipeline/`)

**当前状态**:
```go
// incremental.go - 独立函数和包级变量
var runGenerateFallback = RunFullGenerate
var getChangedFiles = gitdiff.GetChangedFiles
var computeAffected = analyzer.ComputeAffected
// ...

func RunFullGenerate(ctx context.Context, ...) error
func RunIncremental(ctx context.Context, ...) error
```

**重构建议**:
```go
// Pipeline 文档生成流水线
type Pipeline struct {
    config *configpkg.Config
    client llm.Client
    logger logpkg.Logger

    // 可替换的组件
    analyzer     Analyzer
    planner      Planner
    preprocessor Preprocessor
    agent        AgentOrchestrator
    composer     Composer
}

func NewPipeline(cfg *configpkg.Config, client llm.Client, logger logpkg.Logger) *Pipeline {
    return &Pipeline{
        config: cfg,
        client: client,
        logger: logger,
    }
}

func (p *Pipeline) RunFull(ctx context.Context) error {
    // Phase 1: Analysis
    if err := p.runPhase1(ctx); err != nil {
        return err
    }

    // Phase 2: Planning
    plan, err := p.runPlanning(ctx)
    if err != nil {
        return err
    }

    // Phase 3: Preprocessing
    sharedCtx, err := p.runPreprocessing(ctx, plan)
    if err != nil {
        return err
    }

    // Phase 4: Agent
    if err := p.runAgent(ctx, plan, sharedCtx); err != nil {
        return err
    }

    // Phase 5: Composition
    return p.runComposition(ctx, plan)
}

func (p *Pipeline) RunIncremental(ctx context.Context, opts IncrementalOptions) error {
    // 实现
}

// IncrementalOptions 增量更新选项
type IncrementalOptions struct {
    BaseRef      string
    HeadRef      string
    ChangedFiles string
}
```

---

## 五、代码异味与改进

### 5.1 包级别状态

**问题文件**:
- `internal/planner/planner.go:12` - `var logger`
- `internal/planner/skeleton.go:12` - `var logger`
- `internal/preprocessor/shared_context.go:11` - `var sharedLogger`
- `internal/analyzer/analyzer.go:15` - `var registry`

**修复方案**:
```go
// 使用依赖注入替代全局变量
type Planner struct {
    client llm.Client
    logger logpkg.Logger
    config *configpkg.Config
}

func NewPlanner(client llm.Client, config *configpkg.Config, logger logpkg.Logger) *Planner {
    return &Planner{
        client: client,
        config: config,
        logger: logger,
    }
}
```

### 5.2 函数过长

**问题**: `internal/pipeline/incremental.go:RunFullGenerate` 函数包含多个阶段逻辑

**修复方案**: 将每个阶段提取为独立方法

### 5.3 重复代码

**问题**: `cmd/wikismit/*.go` 中存在重复的客户端创建逻辑

**修复方案**:
```go
// helpers.go
type ClientFactory struct {
    config *configpkg.Config
}

func NewClientFactory(config *configpkg.Config) *ClientFactory {
    return &ClientFactory{config: config}
}

func (f *ClientFactory) Create() (llm.Client, error) {
    return llm.NewClient(f.config.LLM)
}
```

### 5.4 魔法数字

**问题**:
- `internal/llm/retry.go:71-74` - `2 * time.Second`, `30 * time.Second`
- `internal/config/config.go:69-70` - `4096`, `0.2`, `120`

**修复方案**: 定义为常量或配置项

```go
const (
    DefaultRetryBaseDelay = 2 * time.Second
    DefaultRetryMaxDelay  = 30 * time.Second
    DefaultMaxTokens      = 4096
    DefaultTemperature    = 0.2
    DefaultTimeoutSeconds = 120
)
```

---

## 六、重构优先级与计划

### Phase 1: 基础重构 (Critical)

| 优先级 | 任务 | 文件 | 原因 |
|--------|------|------|------|
| P0 | 消除全局变量 | planner.go, skeleton.go, shared_context.go | 并发安全 |
| P0 | 修复无限循环 | llm/client.go | 正确性 |
| P0 | 路径验证 | build.go, validate.go | 安全 |

### Phase 2: 核心对象化 (Important)

| 优先级 | 任务 | 包 | 原因 |
|--------|------|------|------|
| P1 | Agent 包重构 | internal/agent | 可测试性 |
| P1 | Pipeline 包重构 | internal/pipeline | 可维护性 |
| P1 | Composer 包重构 | internal/composer | 代码组织 |

### Phase 3: 接口抽象 (Enhancement)

| 优先级 | 任务 | 描述 |
|--------|------|------|
| P2 | 定义领域接口 | 为核心组件定义接口 |
| P2 | 实现依赖注入 | 使用构造函数注入依赖 |
| P2 | 添加验证逻辑 | 为配置和请求添加验证方法 |

---

## 七、推荐接口定义

```go
// internal/interfaces/interfaces.go

// DocumentGenerator 文档生成器接口
type DocumentGenerator interface {
    Generate(ctx context.Context, module store.Module) (ModuleDoc, error)
}

// CodeAnalyzer 代码分析器接口
type CodeAnalyzer interface {
    Analyze(repoPath string) (store.FileIndex, error)
    BuildDependencyGraph(idx store.FileIndex) store.DepGraph
}

// Planner 规划器接口
type Planner interface {
    Plan(ctx context.Context, idx store.FileIndex, graph store.DepGraph) (*store.NavPlan, error)
}

// Preprocessor 预处理器接口
type Preprocessor interface {
    Process(ctx context.Context, plan *store.NavPlan, idx store.FileIndex, graph store.DepGraph) (store.SharedContext, error)
    ProcessAffected(ctx context.Context, affected []store.Module, plan *store.NavPlan, idx store.FileIndex, graph store.DepGraph) (store.SharedContext, error)
}

// Composer 组合器接口
type Composer interface {
    Compose(plan *store.NavPlan, idx store.FileIndex, graph store.DepGraph) error
}

// LLMClient LLM客户端接口
type LLMClient interface {
    Complete(ctx context.Context, req CompletionRequest) (string, error)
}
```

---

## 八、测试建议

### 8.1 单元测试改进
- 使用 mock 客户端测试业务逻辑
- 为每个新结构体编写构造函数测试
- 添加边界条件测试

### 8.2 集成测试
- 测试完整流水线执行
- 测试增量更新场景
- 测试错误恢复路径

---

## 九、总结

### 主要改进点

1. **消除全局状态**: 将包级变量改为结构体字段
2. **封装业务逻辑**: 将独立函数组织为方法
3. **定义领域接口**: 提高可测试性和可扩展性
4. **统一错误处理**: 建立一致的错误处理模式
5. **加强安全验证**: 添加路径验证和输入校验

### 预期收益

- **可维护性**: 代码组织更清晰，职责更明确
- **可测试性**: 依赖注入使单元测试更容易
- **可扩展性**: 接口抽象支持不同实现
- **安全性**: 输入验证防止潜在攻击
- **并发安全**: 消除全局状态避免数据竞争
