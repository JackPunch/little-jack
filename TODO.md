# little-jack 改进清单

1. - [x] 增加配置验证，提前暴露缺失的必填项
2. - [x] 定义消息角色的常量，替代裸字符串
3. - [ ] 用构造函数替代 `Chat` 中的 nil 检查
4. - [ ] API 非 200 响应时返回详细错误信息
5. - [ ] `Thinking` 字段应按需发送，避免传 `disabled`
6. - [ ] 手写 `.env` 解析器支持引号值
7. - [ ] `Stream` 配置与 `Chat` 实现不匹配
8. - [ ] 拆分文件，改善包结构
9. - [ ] 提取裸字符串为常量（`.env` 文件名、环境变量键名、HTTP Header、`/chat/completions`、`enabled`/`disabled` 等）

