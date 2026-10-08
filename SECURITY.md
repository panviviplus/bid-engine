# 安全策略

## 报告安全问题

如果发现安全漏洞，请不要直接开公开 Issue，请通过 GitHub 的私密渠道报告：

- 在本仓库 Security 标签页使用 Report a vulnerability（Private vulnerability reporting）
- 或直接联系维护者：https://github.com/panviviplus

请尽量提供：影响范围、复现步骤、涉及的路由或模块、可能的修复建议。我们
会尽快确认并回复。

## 需要重点关注的区域

- 鉴权与令牌：`bid-engine-backend/pkg/middleware`、JWT 与 Cookie 配置
- OpenAPI 通道：`/openapi/*` 路由与 `properties.openapi_token`
- 文件上传与解析：PDF/DOCX 解析链路、对象存储（MinIO）访问控制
- LLM 调用：用户自行配置的模型地址与密钥（`user_llm_config`）可能指向
  任意外部地址，注意 SSRF 与密钥泄露风险

## 部署方须知

本仓库不含任何真实密钥。你自己部署时：

1. 不要提交任何真实密钥、`.env.local` 或生产配置
2. 为 MySQL、Redis、MinIO 设置强口令并限制网络可达范围
3. 如启用 OpenAPI 通道，请配置高强度的 `properties.openapi_token`；留空即
   关闭该通道
4. 及时清理 `bid-engine-backend/logs/`、`bid-engine-backend/tmp/` 中的日志与
   解析中间产物，避免业务数据外泄
