# 调用模式矩阵（Go SDK）

本文对应你关心的三类使用模式，并给出可直接运行的 example。

## 1. 模式总览

| 模式 | 入口 | 特点 | 适用场景 | 示例 |
|---|---|---|---|---|
| A. 一次性请求 | `Query(ctx, prompt, ...)` | 单次输入，流式输出 | 一问一答、脚本任务 | `examples/matrix_query_once/main.go` |
| B. 多次输入流 | `QueryStream(ctx, inputCh, ...)` | 多条输入入队，CLI 按顺序处理 | 批量问题、流水线输入 | `examples/matrix_query_stream/main.go` |
| C. 长连接会话 | `NewClient + Connect + QueryWithSession` | 同一连接下多 session 会话 | 聊天 UI、多会话路由 | `examples/matrix_client_sessions/main.go` |

## 2. 关键行为（非常重要）

1. 输出不是等输入关闭后才出现  
`QueryStream` 模式下，输入与输出是并发的，CLI 会边处理边返回消息。

2. 普通 response 不提供 `parentUuid`  
`parentUuid` 这类字段存在于本地 transcript（`~/.claude/projects/...jsonl`），不在 SDK 标准流消息里。

3. 实战建议  
如果你不依赖 transcript 做关联，推荐以 `ResultMessage` 作为每一轮的结束边界；同一 `session_id` 下串行发送最稳。

## 3. 运行示例

在仓库根目录执行：

```bash
go run ./examples/matrix_query_once
go run ./examples/matrix_query_stream
go run ./examples/matrix_client_sessions
```

## 4. 常用可选项

1. 指定模型：`WithModel("sonnet")` / `WithModel("opus")`
2. 指定权限：`WithPermissionMode(PermissionDefault)`
3. 限制轮数：`WithMaxTurns(n)`
4. 指定 CLI 路径：`WithCLIPath("~/bin/claude")`（按你的环境可改绝对路径）
