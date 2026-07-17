## P1 — 核心功能 / 用户体验（尽快完成，让程序真正可用）

- [ ] **拆分文件**：将 main.go 按职责拆为 main.go / config.go / types.go / agent.go / tools.go，同一 package main。当前所有逻辑挤在单文件，越早拆后续改动成本越低
- [ ] **完善工具调用**：当前工具调用仅在 main 循环中硬编码 `MockTool.findNews`，需实现通用工具注册机制、支持多轮 tool call 循环、`tool_choice` 参数控制
- [ ] **修复工具调用后出错导致死循环**：当 `isToolCalling=true` 时若 `Chat()` 失败，代码弹出最后一条消息后 `continue`，但 `isToolCalling` 未重置为 `false`，导致下一轮跳过用户输入再次调用 API，陷入无限循环。应在出错时将 `isToolCalling` 置 `false` 并清理已追加的 tool result 消息
- [ ] **完善回退机制**：当前 API 调用失败仅简单弹出最后一条用户消息，需进一步考虑重试策略、失败提示及历史状态一致性
- [ ] **增加退出指令与输入优化**：支持输入 `exit` / `quit` 优雅退出
- [ ] **控制对话上下文长度**：目前 `messages` 无限增长，长对话会触发 API token 上限。后续需按轮数或 token 截断历史
- [ ] **支持流式输出**：`Config.Stream` 已存在但 Chat() 未实现 SSE 解析，需按 OpenAI SSE 格式逐块输出 delta。大模型交互的基础体验，与 P0 崩溃 bug 联动
- [ ] **等待 API 响应时阻塞用户输入**：当前主循环中 `Chat()` 为同步阻塞调用，期间用户仍可输入但程序不会响应。需在请求期间禁止用户输入，或在响应到达前不读取 stdin（或改为异步+提示）。严重影响交互体验
- [ ] **`Chat()` 方法签名添加 `context.Context`**：支持请求取消和超时控制，替代全局 150s
- [ ] **API 非 200 响应时返回详细错误信息**：目前只返回 status code，应带上 response body 便于排障
- [ ] **修复 URL 拼接双斜杠**：`BaseURL` 尾部带 `/` 时会产生 `//chat/completions`，需 `strings.TrimSuffix` 处理

## P2 — 工程优化 / 代码质量（越早做成本越低）

- [ ] **利用 `Debug` 字段输出调试信息**：目前注册了但 nowhere 使用，至少打印 request/response body
- [ ] **提取裸字符串为常量**：`.env` 文件名、环境变量键名、HTTP Header 键、`/chat/completions`、`enabled`/`disabled` 等
- [ ] **`Thinking` 字段用值类型替代指针**：当前总是构造 `&Thinking{}`，指针无意义
- [ ] **定义 `ChatClient` 接口**：方便后续单元测试 mock，不依赖真实 API
- [ ] **`NewAgent` 支持自定义 HTTP Client**（Functional Options）：允许注入代理、自定义 Transport、调整 Timeout

---

- [x] **Config 结构体加标签**：已添加 `yaml` 标签，便于序列化/反序列化
- [x] **支持显示思考内容**：`Message.ReasoningContent` 已定义但 nowhere 输出，需在 `Thinking=true` 时打印 reasoning_content。实现成本低，收益高
- [x] **改用 `config.yaml` 配置文件，并支持交互式设置**：替代 `.env`，YAML 可读性更好且支持注释；若配置文件不存在，交互式提示用户输入必填项并自动创建。配置是入口，越早改后续改动范围越小
- [x] **增加主循环，支持多轮对话**：当前 main() 只执行一次硬编码对话，应改为交互式循环读取用户输入，维护 message 上下文
- [x] **修复主循环错误处理缺陷**：`reader.ReadString` 忽略 EOF 会导致死循环发送空请求；API 错误在循环内使用 `log.Fatal` 会导致程序直接退出。应改为：EOF 时优雅退出、API 错误打印后 `continue`、跳过空输入
- [x] **支持工具调用**：`Config.Tools` 已存在但未处理 tool_calls / tool 角色消息，需解析函数调用并支持把结果回传模型。
- [x] 增加配置验证，提前暴露缺失的必填项
- [x] 用构造函数替代 `Chat` 中的 config/client nil 检查
- [x] 定义消息角色的常量，替代裸字符串
- [x] **去掉 `Chat()` 中 `agent == nil` 的检查**：Go 共识里 receiver nil 是调用方责任，不必防御
