# Plan 01 — 实现 BuildPlannerSkeleton

**Epic ref:** S9C.1
**Depends on:** none

---

## Goal

在 `internal/planner/skeleton.go` 中新增 `BuildPlannerSkeleton(idx store.FileIndex, maxTokens int) string`，输出极简 skeleton（类型名 + 内部 import），不含函数签名。

---

## Implementation steps

### Step 1: 实现 BuildPlannerSkeleton 函数

**File:** `internal/planner/skeleton.go`

在 `BuildFullSkeleton` 之后添加新函数：

```go
// BuildPlannerSkeleton 专为 Planner 设计的极简 skeleton。
// 只输出文件路径、exported 类型名和内部 import 关系，
// 不输出函数签名。比 BuildFullSkeleton 压缩约 70-80%。
func BuildPlannerSkeleton(idx store.FileIndex, maxTokens int) string {
    sortedFiles := make([]string, 0, len(idx))
    for file := range idx {
        sortedFiles = append(sortedFiles, file)
    }
    sort.Strings(sortedFiles)

    var lines []string
    chars := 0

    for _, file := range sortedFiles {
        entry, ok := idx[file]
        if !ok {
            continue
        }

        // 为当前文件构建所有输出行
        var fileLines []string
        fileChars := 0

        // 文件路径头
        header := fmt.Sprintf("// %s", file)
        fileLines, fileChars = appendLineWithCharCount(fileLines, fileChars, header)

        // Exported 类型名（逗号分隔在一行）
        var typeNames []string
        for _, typ := range entry.Types {
            if typ.Exported {
                typeNames = append(typeNames, typ.Name)
            }
        }
        if len(typeNames) > 0 {
            typeLine := fmt.Sprintf("  type %s", strings.Join(typeNames, ", "))
            fileLines, fileChars = appendLineWithCharCount(fileLines, fileChars, typeLine)
        }

        // 内部 import 关系
        var importPaths []string
        for _, imp := range entry.Imports {
            if imp.Internal && imp.ResolvedPath != "" {
                importPaths = append(importPaths, imp.ResolvedPath)
            }
        }
        if len(importPaths) > 0 {
            importLine := fmt.Sprintf("  -> %s", strings.Join(importPaths, ", "))
            fileLines, fileChars = appendLineWithCharCount(fileLines, fileChars, importLine)
        }

        // 按文件粒度检查 token 预算
        wouldExceed := false
        testChars := chars
        for _, l := range fileLines {
            if estimatedTokensAfterAppend(testChars, l) > maxTokens {
                wouldExceed = true
                break
            }
            testChars += len(l)
            if testChars > 0 {
                testChars++ // newline
            }
        }

        if wouldExceed {
            if logger != nil {
                logger.Warn("planner skeleton truncated", "file", file)
            }
            break // 停止添加更多文件
        }

        // 整个文件放入
        for _, l := range fileLines {
            lines, chars = appendLineWithCharCount(lines, chars, l)
        }
    }

    return strings.Join(lines, "\n")
}
```

**关键设计决策：**
- 文件粒度截断：整个文件要么完整放入，要么跳过。不截断单文件内容，保证 Planner 看到的每个文件信息完整。
- 类型名逗号分隔在一行，减少行数。
- `->` 前缀标识 import 行，与类型行（`type`）和路径头（`//`）视觉区分。

---

### Step 2: 编写单元测试

**File:** `internal/planner/skeleton_test.go`

新增以下测试用例：

#### Test 2a: BuildPlannerSkeleton 基本输出格式

- 构建包含 2-3 个文件的 `FileIndex`，包含类型和内部 import
- 验证输出包含文件路径头（`//` 前缀）
- 验证 exported 类型出现在 `type` 行
- 验证内部 import 出现在 `->` 行
- 验证不包含任何函数签名

#### Test 2b: BuildPlannerSkeleton 不含函数签名

- 构建包含函数和类型的 `FileIndex`
- 验证输出中不含任何函数名或函数签名

#### Test 2c: BuildPlannerSkeleton token 截断

- 构建大量文件的 `FileIndex`（超过 `maxTokens`）
- 验证 `estimateTokens(result) <= maxTokens`
- 验证被包含的文件信息完整（不是部分截断）

#### Test 2d: BuildPlannerSkeleton 空文件占位

- 构建包含无类型无内部 import 的文件的 `FileIndex`
- 验证该文件仍然出现在输出中（只有路径头行）

#### Test 2e: BuildPlannerSkeleton 忽略外部 import

- 构建包含外部 import（`Internal: false`）的文件
- 验证外部 import 不出现在 `->` 行中

---

## Verification

```bash
go test ./internal/planner -run BuildPlannerSkeleton -v
```

所有新增测试通过，现有 `BuildSkeleton` / `BuildFullSkeleton` 测试不受影响。
