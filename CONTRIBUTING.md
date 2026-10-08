# 参与贡献

欢迎在 main 分支的基础上提出问题、提交改进。参与本项目即表示你同意下方的贡献条款。

## 可以做的事

- 提 Issue：报告缺陷、提出功能建议、指出文档问题
- 提 Pull Request：修复缺陷、改进实现、补充文档
- Fork 后在 main 基础上继续开发（非商业用途）

## 提 Issue

请尽量包含：复现步骤、期望结果与实际结果、运行模式（本地 `start.sh` 或容器部署）、相关日志片段、版本或提交号。

**不要**在 Issue 或评论里粘贴任何真实密钥、账号口令、招标文件原文或客户数据。日志和截图请先脱敏。

## 提 Pull Request

1. 从 `main` 切出分支，分支名建议 `fix/xxx` 或 `feat/xxx`
2. 保持改动聚焦：一个 PR 解决一个问题，避免混入无关的格式化改动
3. 后端改动请确保 `cd bid-engine-backend && go build ./...` 通过
4. 前端改动请确保编译检查通过（`npm install --legacy-peer-deps` 后执行 `npx tsc --noEmit` 或 `npm run build`）
5. 提交信息沿用现有风格：`type: 中文描述`（feat / fix / refactor / docs / chore）
6. PR 描述里说明动机、实现方式、验证方式；涉及界面调整的请附截图
7. 不要在 PR 中提交密钥、`.env.local`、本地日志、真实业务文档

## 贡献条款

本项目采用 PolyForm Noncommercial License 1.0.0（见 [LICENSE](./LICENSE)），仅允许非商业用途。

为避免歧义，你提交 Pull Request 即表示：

1. 你确认自己有权提交这些代码，且不侵犯任何第三方权利；
2. 你同意你的贡献同样按 PolyForm Noncommercial License 1.0.0 授权给本项目；
3. 你额外授权项目维护者（panviviplus）在必要的时候，以其他许可证（包括商业许可证）再分发你的贡献内容，以便项目进行双授权或调整授权方式。

如果无法接受第 3 条，请不要提交 PR，改为在 Issue 里描述你的改进思路。

## 商标

项目名、Logo 与视觉素材不在许可范围内，详见 [NOTICE](./NOTICE)。
