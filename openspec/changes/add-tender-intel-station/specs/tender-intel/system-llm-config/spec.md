# Spec Delta

## Purpose

为平台级后台任务（招标情报采集打标等）提供一套与用户无关的模型配置：由系统超管维护候选配置列表，后台任务按固定顺序取用并在失败时自动降级。

## ADDED Requirements

### Requirement: 全局模型配置列表

系统 SHALL 提供全局模型配置存储，每条配置 SHALL 包含名称、base_url、api_key、model、endpoint_path、上下文窗口、最大输出与排序值 sort。系统 SHALL NOT 将全局配置与用户级模型配置混用。

#### Scenario: 超管新增全局配置

- **WHEN** 系统超管提交一条全局模型配置
- **THEN** 配置被保存且可在列表中查询，api_key 在返回时脱敏

#### Scenario: 非超管访问

- **WHEN** 非系统超管用户请求全局模型配置的写接口
- **THEN** 系统拒绝请求并返回无权限错误

### Requirement: 候选选取与降级

后台任务 SHALL 按 sort 升序选取全局模型配置，sort 相同时取最新创建的一条；字段不完整（base_url、api_key、model、endpoint_path 任一为空）的配置 SHALL 被跳过。

#### Scenario: 跳过字段不完整的配置

- **WHEN** sort 最小的配置缺少 model 字段
- **THEN** 后台任务选取下一条字段完整的配置

#### Scenario: 调用失败自动降级

- **WHEN** 选取的配置在调用时返回错误
- **THEN** 任务按顺序尝试下一个候选配置，并在日志中记录失败原因

#### Scenario: 无任何可用配置

- **WHEN** 全局配置表为空或全部不可用
- **THEN** 后台任务回退到配置文件中的默认模型配置

### Requirement: 后台任务不因模型不可用而丢数据

当模型配置全部不可用导致打标失败时，系统 SHALL 保留已入库的公告并标记为未打标状态，供后续重试，SHALL NOT 丢弃已抓取的公告数据。

#### Scenario: 打标失败后重试

- **WHEN** 某轮采集因模型不可用导致打标失败
- **THEN** 公告仍存在于情报库中，且可在后续轮次或手动触发时补打标签

### Requirement: 连通性测试

系统 SHALL 提供全局模型配置的连通性测试能力，测试 SHALL NOT 持久化任何业务数据。

#### Scenario: 测试可用配置

- **WHEN** 超管对某条配置发起连通性测试
- **THEN** 系统返回调用是否成功以及失败原因
