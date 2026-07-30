# static/dist Git 跟踪规则讨论记录

## 原始需求

`static/dist` 目录中的前端编译文件此前已经被加入 Git。希望以后只提交并保留 `static/dist/.placeholder`，其他编译文件全部忽略，从而避免每次前端编译都产生该目录的 Git 变更。

## 问题与回答

### 问题 1

是否采用“只从 Git 索引移除编译文件，但保留本地文件”的方式？

推荐答案：是。这样既能停止跟踪编译产物，又不会删除本地构建结果。

用户回答：确认。

### 问题 2

是否只让编译文件从当前版本开始停止跟踪，不重写以前的 Git 提交历史？

推荐答案：是。历史提交中的旧文件不会影响后续状态，避免重写历史导致 commit ID 变化和协作风险。

用户回答：确认。

### 问题 3

是否继续以 `make build`、`make build-lin`、`make build-lin-arm` 作为正式构建入口，并由这些目标先执行前端构建？

推荐答案：是。正式二进制通过现有 `make build*` 流程生成并内嵌前端；直接执行 `go build` 不作为正式构建方式。

用户回答：确认。

### 问题 4

是否将忽略规则简化为忽略 `static/dist` 的所有内容，仅放行 `.placeholder`？

推荐答案：是。删除 `index.html` 的放行规则和重复的 `assets` 忽略规则，使规则更明确。

用户回答：确认。

### 问题 5

是否不新增自动化测试，只通过 Git 跟踪行为、忽略规则和 `make web` 构建结果进行验收？

推荐答案：是。本次没有业务代码或复杂状态转换，不需要新增单元测试。

用户回答：确认。

## 最终共同理解

- `static/dist/.placeholder` 是该目录唯一允许被 Git 跟踪的文件。
- 当前已经跟踪的 `index.html`、`favicon.svg` 和 `assets` 编译文件只从 Git 索引中移除，本地文件继续保留。
- 不重写已有 Git 历史，从当前提交开始停止跟踪编译产物。
- 忽略规则统一为忽略 `static/dist` 下全部内容并仅放行 `.placeholder`。
- `make web` 继续负责清理并重新生成 `static/dist`，同时创建 `.placeholder`。
- 正式构建继续使用 `make build*`，保证前端先构建再被 Go `embed` 内嵌。
- 验收关注 `git ls-files`、`git check-ignore`、`make web` 后的 `git status` 和本地构建文件是否仍存在。
