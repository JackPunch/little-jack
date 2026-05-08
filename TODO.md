# little-jack 改进清单

> 轻量级本地 Issue 管理，完成一项勾一项。

---

## 1. 增加配置验证，提前暴露缺失的必填项

**描述**：目前 `getConfig()` 读取 `.env` 后不会检查必填字段。漏配 `API_KEY` 时程序会在 HTTP 阶段才报错，信息不直观。

**验收标准**：
- [ ] 给 `Config` 添加 `Validate() error` 方法
- [ ] 验证 `BaseURL`、`APIKey`、`ModelName` 去除首尾空格后不为空
- [ ] `main()` 中调用 `config.Validate()`，失败时用 `log.Fatalf` 输出清晰错误
- [ ] 手动测试：删除 `.env` 中的 `API_KEY`，运行应输出 `config invalid: API_KEY is required`

---

## 2. 手写 `.env` 解析器支持引号值

**描述**：当前解析器不支持带引号的值（如 `KEY="hello world"`）。扩展它以加深对 Go 字符串处理的理解。

**验收标准**：
- [ ] 支持双引号值：`KEY="hello world"` → `hello world`
- [ ] 支持单引号值：`KEY='hello world'` → `hello world`
- [ ] 无引号的值保持现有行为
- [ ] 在 `.env` 中添加带空格的引号值进行测试

**可选挑战**：
- [ ] 支持行尾 `\` 续行（多行值）

---

## 3. 用构造函数替代 `Chat` 中的 nil 检查

**描述**：`Agent.Chat()` 有三重 nil 检查。Go 惯用法是通过构造函数保证对象有效，而非运行时防御。

**验收标准**：
- [ ] 新增 `NewAgent(cfg *Config) (*Agent, error)`
- [ ] 构造函数中检查 `cfg != nil`
- [ ] 构造函数中创建 `http.Client` 并设置超时
- [ ] 移除 `Chat` 中的所有 nil 检查
- [ ] `main()` 中使用 `NewAgent(config)` 创建 Agent

---

## 4. API 非 200 响应时返回详细错误信息

**描述**：当前只返回状态码数字（如 `API error 401`），没有响应体详情，调试困难。

**验收标准**：
- [ ] 当 `resp.StatusCode != http.StatusOK` 时读取响应体
- [ ] 错误格式：`API error 401: <响应体内容>`
- [ ] 手动测试：用错误的 API Key，确认能看到详细 401 信息

---

## 5. 定义消息角色的常量，替代裸字符串

**描述**：`Message.Role` 用裸字符串容易拼错，IDE 也无法补全。

**验收标准**：
- [ ] 定义常量：`RoleSystem`、`RoleUser`、`RoleAssistant`
- [ ] `main()` 中的 `messages` 改用常量
- [ ] 运行程序，确认行为无变化

---

## 6. `Thinking` 字段应按需发送，避免传 `disabled`

**描述**：当前始终构造 `Thinking{Type: "disabled"}` 发给 API。未开启时应直接不发送该字段（利用 `omitempty`）。

**验收标准**：
- [ ] `Chat` 中将 `thinking` 指针默认设为 `nil`
- [ ] 仅当 `agent.Config.Thinking == true` 时赋值为 `&Thinking{Type: "enabled"}`
- [ ] 临时打印请求 JSON 验证：关闭 thinking 时请求体不含 `thinking` 字段

---

## 7. `Stream` 配置与 `Chat` 实现不匹配

**描述**：`STREAM=true` 时 `Chat()` 会解析失败，因为它只支持非流式 JSON。

**验收标准**：
- [ ] 在 `Validate()` 或 `Chat()` 中拦截：当 `Stream == true` 时返回明确错误，提示当前不支持

---

## 8. 拆分文件，改善包结构

**描述**：所有代码都在 `main.go` 中，随着功能增加会变臃肿。

**建议拆分**：
- `config.go`：`Config`、`getConfig()`、`Validate()`
- `types.go`：所有数据结构（`Message`、`RequestBody`、`ResponseBody` 等）
- `agent.go`：`Agent`、`NewAgent()`、`Chat()`
- `main.go`：程序入口，组装依赖并启动

**验收标准**：
- [ ] 按上述建议完成拆分
- [ ] `go build` 成功
- [ ] 程序运行结果与拆分前一致

**可选**：
- [ ] 将数据结构拆到独立的 `model` 包，练习包导入

---

## 建议完成顺序

1 → 5 → 3 → 4 → 6 → 2 → 7 → 8

前面改动小、见效快，适合热身；后面逐步提升复杂度。
