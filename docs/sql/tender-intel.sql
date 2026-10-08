-- 招标情报站（Tender Intel Station）建表脚本
--
-- 约定：
--   1. 库：smart-bid（本地 MySQL 8.0）
--   2. 全库禁止外键约束，表间关联仅保留普通字段，一致性由应用层维护
--   3. 字符集统一 utf8mb4 / utf8mb4_bin，存储引擎 InnoDB
--   4. 可重复执行（IF NOT EXISTS / INSERT IGNORE）

-- ============================================================
-- 1. 全局模型配置（系统超管维护，供平台级后台任务使用）
-- ============================================================
CREATE TABLE IF NOT EXISTS `system_llm_config` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '主键',
  `name` varchar(128) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '配置名称，便于管理员识别',
  `base_url` varchar(255) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT 'OpenAI 兼容 API 地址',
  `api_key` varchar(512) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT 'API Key',
  `model` varchar(128) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '模型名称',
  `endpoint_path` varchar(255) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '补全端点路径，默认 /chat/completions',
  `context_window_tokens` int NOT NULL DEFAULT '0' COMMENT '上下文窗口 token 预算，0=使用默认值',
  `max_output_tokens` int NOT NULL DEFAULT '0' COMMENT '单次输出 token 上限，0=使用默认值',
  `sort` int NOT NULL DEFAULT '100' COMMENT '排序值，升序优先；相同取最新创建的一条',
  `create_time` bigint NOT NULL DEFAULT '0' COMMENT '创建时间（秒级时间戳）',
  `update_time` bigint NOT NULL DEFAULT '0' COMMENT '更新时间（秒级时间戳）',
  PRIMARY KEY (`id`),
  KEY `idx_sort_id` (`sort`,`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='全局模型配置表（平台级后台任务使用）';

-- ============================================================
-- 2. 行业枚举
-- ============================================================
CREATE TABLE IF NOT EXISTS `tender_intel_industry` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '主键',
  `code` varchar(64) COLLATE utf8mb4_bin NOT NULL COMMENT '行业编码',
  `name` varchar(64) COLLATE utf8mb4_bin NOT NULL COMMENT '行业名称',
  `sort` int NOT NULL DEFAULT '100' COMMENT '展示排序',
  `enabled` tinyint NOT NULL DEFAULT '1' COMMENT '是否启用：1=启用 0=停用',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`),
  KEY `idx_enabled_sort` (`enabled`,`sort`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='招标情报行业枚举表';

-- ============================================================
-- 3. 采集源与健康度
-- ============================================================
CREATE TABLE IF NOT EXISTS `tender_intel_source` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '主键',
  `source_key` varchar(64) COLLATE utf8mb4_bin NOT NULL COMMENT '源唯一标识，与采集服务 recipe 对应',
  `name` varchar(128) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '源名称',
  `homepage_url` varchar(512) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '门户地址',
  `list_url` varchar(1024) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '列表页地址（覆盖 recipe 默认值）',
  `category` varchar(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '网站性质：国家级/地方级/国央企/银行/高校等',
  `region` varchar(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '地区',
  `industry_hint` varchar(128) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '网站行业属性提示',
  `discovery_mode` varchar(32) COLLATE utf8mb4_bin NOT NULL DEFAULT 'list' COMMENT '发现方式：api/list/browser',
  `needs_browser` tinyint NOT NULL DEFAULT '0' COMMENT '是否需要无头浏览器：1=需要',
  `enabled` tinyint NOT NULL DEFAULT '1' COMMENT '是否启用：1=启用 0=停用',
  `priority` int NOT NULL DEFAULT '100' COMMENT '执行优先级，升序',
  `params` text COLLATE utf8mb4_bin COMMENT '参数覆盖（JSON）',
  `cursor` varchar(512) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '增量游标',
  `last_run_at` datetime DEFAULT NULL COMMENT '最近一次执行时间',
  `last_success_at` datetime DEFAULT NULL COMMENT '最近一次成功时间',
  `last_error` text COLLATE utf8mb4_bin COMMENT '最近一次错误',
  `consecutive_failures` int NOT NULL DEFAULT '0' COMMENT '连续失败次数',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_source_key` (`source_key`),
  KEY `idx_enabled_priority` (`enabled`,`priority`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='招标情报采集源表';

-- ============================================================
-- 4. 采集批次与源明细
-- ============================================================
CREATE TABLE IF NOT EXISTS `tender_intel_collect_schedule` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '主键，固定使用 id=1 的单行配置',
  `cron_expr` varchar(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '0 0 6 * * *' COMMENT '秒级 cron 表达式（秒 分 时 日 月 周）',
  `enabled` tinyint NOT NULL DEFAULT '1' COMMENT '是否启用自动采集：1=启用 0=停用',
  `updated_by` bigint NOT NULL DEFAULT '0' COMMENT '最近一次修改人（系统超管）',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='招标情报自动采集任务配置表';

-- 默认每日 06:00 自动采集一轮
INSERT IGNORE INTO `tender_intel_collect_schedule` (`id`, `cron_expr`, `enabled`, `updated_by`)
  VALUES (1, '0 0 6 * * *', 1, 0);

CREATE TABLE IF NOT EXISTS `tender_intel_collect_run` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '主键',
  `run_id` varchar(64) COLLATE utf8mb4_bin NOT NULL COMMENT '批次 ID',
  `trigger_type` varchar(16) COLLATE utf8mb4_bin NOT NULL DEFAULT 'cron' COMMENT '触发方式：cron/manual',
  `scope` varchar(16) COLLATE utf8mb4_bin NOT NULL DEFAULT 'all' COMMENT '采集范围：all=全部启用源，single=指定源',
  `source_keys` text COLLATE utf8mb4_bin COMMENT '本次采集覆盖的源（JSON 数组；scope=all 时为空）',
  `retry_of` varchar(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '重试来源批次 run_id（非重试为空）',
  `status` varchar(16) COLLATE utf8mb4_bin NOT NULL DEFAULT 'running' COMMENT '状态：running/success/partial/failed',
  `started_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '开始时间',
  `finished_at` datetime DEFAULT NULL COMMENT '结束时间',
  `source_total` int NOT NULL DEFAULT '0' COMMENT '本批次源总数',
  `source_success` int NOT NULL DEFAULT '0' COMMENT '成功源数',
  `source_failed` int NOT NULL DEFAULT '0' COMMENT '失败源数',
  `discovered_count` int NOT NULL DEFAULT '0' COMMENT '发现公告数',
  `extracted_count` int NOT NULL DEFAULT '0' COMMENT '抽取成功数',
  `inserted_count` int NOT NULL DEFAULT '0' COMMENT '新增入库数',
  `duplicate_count` int NOT NULL DEFAULT '0' COMMENT '重复跳过数',
  `enriched_count` int NOT NULL DEFAULT '0' COMMENT '完成打标数',
  `error_summary` text COLLATE utf8mb4_bin COMMENT '错误摘要',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_run_id` (`run_id`),
  KEY `idx_started_at` (`started_at`),
  KEY `idx_status_started` (`status`,`started_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='招标情报采集批次表';

-- 增量升级：老库补列（幂等，重复执行不会报错）
SET @ddl := (SELECT IF(COUNT(*) = 0,
  'ALTER TABLE `tender_intel_collect_run` ADD COLUMN `scope` varchar(16) COLLATE utf8mb4_bin NOT NULL DEFAULT ''all'' COMMENT ''采集范围：all=全部启用源，single=指定源''',
  'DO 0')
  FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'tender_intel_collect_run' AND column_name = 'scope');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ddl := (SELECT IF(COUNT(*) = 0,
  'ALTER TABLE `tender_intel_collect_run` ADD COLUMN `source_keys` text COLLATE utf8mb4_bin COMMENT ''本次采集覆盖的源（JSON 数组）''',
  'DO 0')
  FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'tender_intel_collect_run' AND column_name = 'source_keys');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ddl := (SELECT IF(COUNT(*) = 0,
  'ALTER TABLE `tender_intel_collect_run` ADD COLUMN `retry_of` varchar(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '''' COMMENT ''重试来源批次 run_id''',
  'DO 0')
  FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'tender_intel_collect_run' AND column_name = 'retry_of');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ddl := (SELECT IF(COUNT(*) = 0,
  'ALTER TABLE `tender_intel_collect_run` ADD KEY `idx_status_started` (`status`,`started_at`)',
  'DO 0')
  FROM information_schema.statistics
  WHERE table_schema = DATABASE() AND table_name = 'tender_intel_collect_run' AND index_name = 'idx_status_started');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `tender_intel_run_source` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '主键',
  `run_id` varchar(64) COLLATE utf8mb4_bin NOT NULL COMMENT '批次 ID',
  `source_key` varchar(64) COLLATE utf8mb4_bin NOT NULL COMMENT '源标识',
  `source_name` varchar(128) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '源名称快照',
  `status` varchar(16) COLLATE utf8mb4_bin NOT NULL DEFAULT 'running' COMMENT '状态：running/success/failed',
  `cursor_before` varchar(512) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '执行前游标',
  `cursor_after` varchar(512) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '执行后游标',
  `discovered` int NOT NULL DEFAULT '0' COMMENT '发现数',
  `extracted` int NOT NULL DEFAULT '0' COMMENT '抽取数',
  `inserted` int NOT NULL DEFAULT '0' COMMENT '入库数',
  `skipped` int NOT NULL DEFAULT '0' COMMENT '跳过数',
  `duration_ms` int NOT NULL DEFAULT '0' COMMENT '耗时（毫秒）',
  `error` text COLLATE utf8mb4_bin COMMENT '错误信息',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_run_source` (`run_id`,`source_key`),
  KEY `idx_source_created` (`source_key`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='招标情报采集批次源明细表';

-- ============================================================
-- 5. 招标公告
-- ============================================================
CREATE TABLE IF NOT EXISTS `tender_intel_notice` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '主键',
  `source_key` varchar(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '来源源标识',
  `source_name` varchar(128) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '来源名称',
  `source_category` varchar(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '来源网站性质',
  `origin` varchar(16) COLLATE utf8mb4_bin NOT NULL DEFAULT 'collect' COMMENT '入库来源：collect=自动采集，manual=系统录入',
  `import_batch` varchar(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '手工录入/批量导入的批次号（自动采集为空）',
  `url` varchar(1024) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '原始链接',
  `canonical_url` varchar(1024) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '规范化链接',
  `url_hash` char(64) COLLATE utf8mb4_bin NOT NULL COMMENT '规范化链接 SHA-256，唯一键',
  `external_id` varchar(128) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '来源站公告 ID',
  `title` varchar(512) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '公告标题',
  `publisher` varchar(255) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '采购人',
  `agency` varchar(255) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '代理机构',
  `project_code` varchar(128) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '项目编号',
  `budget_text` varchar(255) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '预算原文',
  `budget_amount` decimal(18,2) DEFAULT NULL COMMENT '预算金额（元）',
  `region_province` varchar(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '省级地区',
  `region_city` varchar(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '市级地区',
  `region_text` varchar(128) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '地区原文',
  `notice_type` varchar(32) COLLATE utf8mb4_bin NOT NULL DEFAULT 'other' COMMENT '公告类型：open_tender公开招标/invite_tender邀请招标/negotiation竞争性磋商/tender_negotiation竞争性谈判/inquiry询价/prequalification资格预审/single_source单一来源/change变更澄清/terminate终止/other其他',
  `notice_stage` varchar(32) COLLATE utf8mb4_bin NOT NULL DEFAULT 'procurement' COMMENT '公告阶段：procurement采购公告/prequalification预审/change变更澄清/other其他',
  `publish_date` date DEFAULT NULL COMMENT '发布日期',
  `publish_at` datetime DEFAULT NULL COMMENT '发布精确时间',
  `deadline_at` datetime DEFAULT NULL COMMENT '投标/响应截止时间',
  `body_html` mediumtext COLLATE utf8mb4_bin COMMENT '清洗后的正文语义 HTML',
  `body_markdown` mediumtext COLLATE utf8mb4_bin COMMENT '正文 Markdown',
  `body_text` mediumtext COLLATE utf8mb4_bin COMMENT '正文纯文本',
  `content_hash` char(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '正文内容哈希',
  `attachments` text COLLATE utf8mb4_bin COMMENT '附件链接（JSON 数组）',
  `fetch_strategy` varchar(32) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '抽取策略',
  `extract_confidence` decimal(5,4) NOT NULL DEFAULT '0.0000' COMMENT '抽取置信度 0-1',
  `extract_warnings` text COLLATE utf8mb4_bin COMMENT '抽取告警（JSON 数组）',
  `tag_status` varchar(16) COLLATE utf8mb4_bin NOT NULL DEFAULT 'pending' COMMENT '打标状态：pending/done/failed',
  `status` varchar(16) COLLATE utf8mb4_bin NOT NULL DEFAULT 'normal' COMMENT '公告状态：normal在架/hidden隐藏/archived下架/duplicate/invalid',
  `pinned` tinyint NOT NULL DEFAULT '0' COMMENT '置顶：1=置顶，0=普通',
  `pinned_at` datetime DEFAULT NULL COMMENT '置顶时间（取消置顶后清空）',
  `admin_note` varchar(255) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '管理备注（隐藏/下架原因等）',
  `first_seen_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '首次发现时间',
  `last_seen_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '最近发现时间',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_url_hash` (`url_hash`),
  KEY `idx_publish_id` (`publish_date`,`id`),
  KEY `idx_notice_type` (`notice_type`),
  KEY `idx_region_province` (`region_province`),
  KEY `idx_source_publish` (`source_key`,`publish_date`),
  KEY `idx_tag_status` (`tag_status`),
  KEY `idx_budget_amount` (`budget_amount`),
  KEY `idx_status_pinned_publish` (`status`,`pinned`,`publish_date`,`id`),
  KEY `idx_origin_batch` (`origin`,`import_batch`),
  KEY `idx_first_seen_at` (`first_seen_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='招标情报公告表';

-- 增量升级：老库为 tender_intel_notice 补齐情报管理字段（幂等，重复执行不会报错）
SET @ddl := (SELECT IF(COUNT(*) = 0,
  'ALTER TABLE `tender_intel_notice` ADD COLUMN `origin` varchar(16) COLLATE utf8mb4_bin NOT NULL DEFAULT ''collect'' COMMENT ''入库来源：collect=自动采集，manual=系统录入'' AFTER `source_category`',
  'DO 0')
  FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'tender_intel_notice' AND column_name = 'origin');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ddl := (SELECT IF(COUNT(*) = 0,
  'ALTER TABLE `tender_intel_notice` ADD COLUMN `body_html` mediumtext COLLATE utf8mb4_bin COMMENT ''清洗后的正文语义 HTML'' AFTER `deadline_at`',
  'DO 0')
  FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'tender_intel_notice' AND column_name = 'body_html');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ddl := (SELECT IF(COUNT(*) = 0,
  'ALTER TABLE `tender_intel_notice` ADD COLUMN `import_batch` varchar(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '''' COMMENT ''手工录入/批量导入的批次号（自动采集为空）'' AFTER `origin`',
  'DO 0')
  FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'tender_intel_notice' AND column_name = 'import_batch');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ddl := (SELECT IF(COUNT(*) = 0,
  'ALTER TABLE `tender_intel_notice` ADD COLUMN `pinned` tinyint NOT NULL DEFAULT ''0'' COMMENT ''置顶：1=置顶，0=普通'' AFTER `status`',
  'DO 0')
  FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'tender_intel_notice' AND column_name = 'pinned');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ddl := (SELECT IF(COUNT(*) = 0,
  'ALTER TABLE `tender_intel_notice` ADD COLUMN `pinned_at` datetime DEFAULT NULL COMMENT ''置顶时间（取消置顶后清空）'' AFTER `pinned`',
  'DO 0')
  FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'tender_intel_notice' AND column_name = 'pinned_at');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ddl := (SELECT IF(COUNT(*) = 0,
  'ALTER TABLE `tender_intel_notice` ADD COLUMN `admin_note` varchar(255) COLLATE utf8mb4_bin NOT NULL DEFAULT '''' COMMENT ''管理备注（隐藏/下架原因等）'' AFTER `pinned_at`',
  'DO 0')
  FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'tender_intel_notice' AND column_name = 'admin_note');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ddl := (SELECT IF(COUNT(*) = 0,
  'ALTER TABLE `tender_intel_notice` ADD KEY `idx_status_pinned_publish` (`status`,`pinned`,`publish_date`,`id`)',
  'DO 0')
  FROM information_schema.statistics
  WHERE table_schema = DATABASE() AND table_name = 'tender_intel_notice' AND index_name = 'idx_status_pinned_publish');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ddl := (SELECT IF(COUNT(*) = 0,
  'ALTER TABLE `tender_intel_notice` ADD KEY `idx_origin_batch` (`origin`,`import_batch`)',
  'DO 0')
  FROM information_schema.statistics
  WHERE table_schema = DATABASE() AND table_name = 'tender_intel_notice' AND index_name = 'idx_origin_batch');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ddl := (SELECT IF(COUNT(*) = 0,
  'ALTER TABLE `tender_intel_notice` ADD KEY `idx_first_seen_at` (`first_seen_at`)',
  'DO 0')
  FROM information_schema.statistics
  WHERE table_schema = DATABASE() AND table_name = 'tender_intel_notice' AND index_name = 'idx_first_seen_at');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `tender_intel_notice_industry` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '主键',
  `notice_id` bigint NOT NULL COMMENT '公告 ID',
  `industry_code` varchar(64) COLLATE utf8mb4_bin NOT NULL COMMENT '行业编码',
  `weight` int NOT NULL DEFAULT '0' COMMENT '权重，越大越相关',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_notice_industry` (`notice_id`,`industry_code`),
  KEY `idx_industry_notice` (`industry_code`,`notice_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='招标情报公告行业标签表';

CREATE TABLE IF NOT EXISTS `tender_intel_notice_insight` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '主键',
  `notice_id` bigint NOT NULL COMMENT '公告 ID',
  `content_md` mediumtext COLLATE utf8mb4_bin COMMENT 'AI 解读内容（Markdown）',
  `model` varchar(128) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '生成使用的模型',
  `status` varchar(16) COLLATE utf8mb4_bin NOT NULL DEFAULT 'succeeded' COMMENT '状态：succeeded/failed',
  `error` text COLLATE utf8mb4_bin COMMENT '失败原因',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_notice_id` (`notice_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='招标情报公告 AI 解读缓存表';

-- ============================================================
-- 6. 订阅、提醒与收藏
-- ============================================================
CREATE TABLE IF NOT EXISTS `tender_intel_subscription` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '主键',
  `user_id` bigint NOT NULL DEFAULT '0' COMMENT '订阅用户 ID',
  `name` varchar(128) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '订阅名称',
  `keywords` text COLLATE utf8mb4_bin COMMENT '关键词（JSON 数组）',
  `match_mode` varchar(8) COLLATE utf8mb4_bin NOT NULL DEFAULT 'any' COMMENT '匹配模式：any任一/all全部',
  `industries` text COLLATE utf8mb4_bin COMMENT '行业标签（JSON 数组）',
  `regions` text COLLATE utf8mb4_bin COMMENT '地区（JSON 数组）',
  `notice_types` text COLLATE utf8mb4_bin COMMENT '公告类型（JSON 数组）',
  `budget_min` decimal(18,2) DEFAULT NULL COMMENT '预算下限（元）',
  `budget_max` decimal(18,2) DEFAULT NULL COMMENT '预算上限（元）',
  `enabled` tinyint NOT NULL DEFAULT '1' COMMENT '是否启用：1=启用 0=停用',
  `last_matched_at` datetime DEFAULT NULL COMMENT '最近命中时间',
  `matched_count` int NOT NULL DEFAULT '0' COMMENT '累计命中数',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_user_enabled` (`user_id`,`enabled`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='招标情报订阅规则表';

CREATE TABLE IF NOT EXISTS `tender_intel_alert` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '主键',
  `user_id` bigint NOT NULL DEFAULT '0' COMMENT '接收用户 ID',
  `subscription_id` bigint NOT NULL DEFAULT '0' COMMENT '订阅规则 ID',
  `notice_id` bigint NOT NULL DEFAULT '0' COMMENT '公告 ID',
  `subscription_name` varchar(128) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '订阅名称快照',
  `notice_title` varchar(512) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '公告标题快照',
  `matched_keywords` varchar(512) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '命中的关键词（JSON 数组）',
  `matched_reason` varchar(512) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '命中原因说明',
  `is_read` tinyint NOT NULL DEFAULT '0' COMMENT '是否已读：1=已读',
  `read_at` datetime DEFAULT NULL COMMENT '读取时间',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_sub_notice` (`user_id`,`subscription_id`,`notice_id`),
  KEY `idx_user_read_created` (`user_id`,`is_read`,`created_at`),
  KEY `idx_notice_id` (`notice_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='招标情报站内提醒表';

CREATE TABLE IF NOT EXISTS `tender_intel_favorite` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '主键',
  `user_id` bigint NOT NULL DEFAULT '0' COMMENT '用户 ID',
  `notice_id` bigint NOT NULL DEFAULT '0' COMMENT '公告 ID',
  `note` varchar(512) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '备注',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_notice` (`user_id`,`notice_id`),
  KEY `idx_user_created` (`user_id`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='招标情报收藏表';

-- ============================================================
-- 7. 基础数据：行业枚举
-- ============================================================
INSERT IGNORE INTO `tender_intel_industry` (`code`, `name`, `sort`, `enabled`) VALUES
  ('it_informatization', 'IT/信息化', 10, 1),
  ('government', '政务', 20, 1),
  ('healthcare', '医疗', 30, 1),
  ('education', '教育', 40, 1),
  ('finance', '金融', 50, 1),
  ('energy', '能源', 60, 1),
  ('construction', '建筑', 70, 1),
  ('transportation', '交通', 80, 1),
  ('manufacturing', '制造', 90, 1),
  ('telecom', '通信', 100, 1),
  ('water', '水利', 110, 1),
  ('environment', '环保', 120, 1),
  ('military', '军队', 130, 1),
  ('other', '其他', 999, 1);

-- ============================================================
-- 8. 基础数据：首批 12 个采集源
--    说明：list_url 与 params 会覆盖采集服务 recipe 中的默认值；
--    上游站点改版时优先改 recipe（走代码评审），DB 只做启停与参数微调。
--    第二批新增源见 8.1。
-- ============================================================
INSERT IGNORE INTO `tender_intel_source`
  (`source_key`, `name`, `homepage_url`, `list_url`, `category`, `region`, `industry_hint`, `discovery_mode`, `needs_browser`, `enabled`, `priority`) VALUES
  ('ccgp_central', '中国政府采购网·中央公告', 'http://www.ccgp.gov.cn/', 'http://www.ccgp.gov.cn/cggg/zygg/', '国家级', '全国', '通用', 'list', 0, 1, 10),
  ('bj_ggzy', '北京市公共资源交易服务平台', 'https://ggzyfw.beijing.gov.cn/', 'https://ggzyfw.beijing.gov.cn/jyxxcggg/index.html', '地方级', '北京市', '通用', 'list', 0, 1, 20),
  ('sh_ggzy', '上海市公共资源交易中心', 'https://www.shggzy.com/', 'https://www.shggzy.com/search/queryContents.jhtml?channelId=2481', '地方级', '上海市', '通用', 'list', 0, 1, 30),
  ('zj_ggzy', '浙江省公共资源交易服务平台', 'https://ggzy.zj.gov.cn/', 'https://ggzy.zj.gov.cn/jyxxgk/list.html?cate=工程建设&catenum=002001&mycatenum=002001001', '地方级', '浙江省', '通用', 'list', 0, 1, 40),
  ('national_ggzy', '全国公共资源交易平台', 'https://www.ggzy.gov.cn/', 'https://www.ggzy.gov.cn/deal/dealList.html', '国家级', '全国', '通用', 'browser', 1, 1, 50),
  ('sc_ggzy', '四川省公共资源交易信息网', 'https://ggzyjy.sc.gov.cn/', 'https://ggzyjy.sc.gov.cn/jyxx/002001/transactionInfo.html', '地方级', '四川省', '通用', 'browser', 1, 1, 60),
  ('csg_bidding', '中国南方电网采购平台', 'https://www.bidding.csg.cn/', 'https://www.bidding.csg.cn/zbcg/index.jhtml', '国央企', '全国', '电力', 'list', 0, 1, 70),
  ('sgcc_ecp', '国家电网电子商务平台', 'https://ecp.sgcc.com.cn/', 'https://ecp.sgcc.com.cn/ecp2.0/portal/', '国央企', '全国', '电力', 'browser', 1, 1, 80),
  ('cmcc_b2b', '中国移动采购与招标网', 'https://b2b.10086.cn/', 'https://b2b.10086.cn/', '国央企', '全国', '通信', 'browser', 1, 1, 90),
  ('cdb_cg', '国家开发银行采购平台', 'https://cg.cdb.com.cn/', 'https://cg.cdb.com.cn/cmscaigou/index.html', '银行', '全国', '金融', 'list', 0, 1, 100),
  ('psbc_cg', '中国邮政储蓄银行采购平台', 'https://cg.11185.cn/', 'https://cg.11185.cn/zgyzcms/category/bulletinList.html?dates=300&categoryId=88&page=1', '银行', '全国', '金融', 'list', 0, 1, 110),
  ('jiangnan_bidding', '江南大学招标采购', 'https://bidding.jiangnan.edu.cn/', 'https://bidding.jiangnan.edu.cn/sfw_cms/e?page=cms.psms.gglist&typeDetail=YXGK&categoryId=4802', '高校', '江苏省', '高校', 'list', 0, 1, 120);

-- ============================================================
-- 8.1 基础数据：新增采集源（第二批）
--    说明：与采集服务 recipes/*.json 一一对应。
--    上游站点改版时仍优先改 recipe（走代码评审），DB 只做启停与参数微调。
-- ============================================================
INSERT IGNORE INTO `tender_intel_source`
  (`source_key`, `name`, `homepage_url`, `list_url`, `category`, `region`, `industry_hint`, `discovery_mode`, `needs_browser`, `enabled`, `priority`) VALUES
  ('ln_ggzy', '辽宁省公共资源交易服务平台', 'https://ggzy.ln.gov.cn/', 'https://ggzy.ln.gov.cn/was5/web/search?page=1&perpage=20&channelid=211892&docchannel=149565,149559&orderby=-DOCRELTIME', '地方级', '辽宁省', '通用', 'api', 0, 1, 130),
  ('hubei_ggzy', '湖北省公共资源交易平台', 'https://jycg.hubei.gov.cn/', 'https://jycg.hubei.gov.cn/jyxx/', '地方级', '湖北省', '通用', 'list', 1, 1, 140),
  ('plap_juncai', '军队采购网', 'https://www.plap.mil.cn/', 'https://www.plap.mil.cn/freecms-glht/site/juncai/cggg/index.html', '国家级', '全国', '军队', 'list', 1, 1, 160),
  ('cecbid', '中国招标投标网', 'https://www.cecbid.org.cn/', 'https://www.cecbid.org.cn/tenders/list', '国家级', '全国', '通用', 'list', 0, 1, 180);

-- ============================================================
-- 8.2 基础数据：全国省级公共资源交易 / 政府采购平台（第三批）
--    说明：与采集服务 recipes/*.json 一一对应；list_url 为各平台
--    “交易信息 / 采购公告”栏目，详情页链接规则见 recipe 的 linkPattern。
--    本轮已覆盖：天津、河北、山西、吉林、江苏、安徽、江西、河南、
--    广西、重庆、贵州、西藏、甘肃、青海、新疆、陕西。
-- ============================================================
INSERT IGNORE INTO `tender_intel_source`
  (`source_key`, `name`, `homepage_url`, `list_url`, `category`, `region`, `industry_hint`, `discovery_mode`, `needs_browser`, `enabled`, `priority`) VALUES
  ('tj_ggzy', '天津市公共资源交易平台', 'https://ggzy.zwfwb.tj.gov.cn/', 'https://ggzy.zwfwb.tj.gov.cn/', '地方级', '天津市', '通用', 'list', 1, 1, 200),
  ('gx_ggzy', '广西壮族自治区公共资源交易平台', 'http://ggzy.jgswj.gxzf.gov.cn/', 'http://ggzy.jgswj.gxzf.gov.cn/', '地方级', '广西壮族自治区', '通用', 'list', 1, 1, 210),
  ('gz_ggzy', '贵州省公共资源交易网', 'https://ggzy.guizhou.gov.cn/', 'https://ggzy.guizhou.gov.cn/', '地方级', '贵州省', '通用', 'list', 1, 1, 220),
  ('qh_ggzy', '青海公共资源交易网', 'http://www.qhggzyjy.gov.cn/ggzy/', 'http://www.qhggzyjy.gov.cn/ggzy/', '地方级', '青海省', '通用', 'list', 1, 1, 230),
  ('js_ggzy', '江苏省公共资源交易平台', 'http://jsggzy.jszwfw.gov.cn/', 'http://jsggzy.jszwfw.gov.cn/jyxx/tradeInfonew.html', '地方级', '江苏省', '通用', 'list', 1, 1, 240),
  ('jx_ggzy', '江西省公共资源交易平台', 'https://www.jxsggzy.cn/', 'https://www.jxsggzy.cn/jyxx/trade.html', '地方级', '江西省', '通用', 'list', 1, 1, 250),
  ('xj_ggzy', '新疆公共资源交易网', 'https://ggzy.xinjiang.gov.cn/', 'https://ggzy.xinjiang.gov.cn/xinjiangggzy_new/jyxx/trade_info.html', '地方级', '新疆维吾尔自治区', '通用', 'list', 1, 1, 260),
  ('cq_ggzy', '重庆市公共资源交易网', 'https://www.cqggzy.com/', 'https://www.cqggzy.com/trade/014002?categoryNum=014002001', '地方级', '重庆市', '通用', 'list', 1, 1, 270),
  ('ccgp_shanxi', '山西政府采购网', 'http://www.ccgp-shanxi.gov.cn/', 'http://www.ccgp-shanxi.gov.cn/', '地方级', '山西省', '通用', 'list', 1, 1, 280),
  ('ccgp_anhui', '安徽政府采购网', 'https://www.ccgp-anhui.gov.cn/', 'https://www.ccgp-anhui.gov.cn/', '地方级', '安徽省', '通用', 'list', 1, 1, 290),
  ('ccgp_jilin', '吉林省政府采购网', 'http://www.ccgp-jilin.gov.cn/', 'http://www.ccgp-jilin.gov.cn/', '地方级', '吉林省', '通用', 'list', 1, 1, 300),
  ('ccgp_guizhou', '贵州省政府采购网', 'http://www.ccgp-guizhou.gov.cn/', 'http://www.ccgp-guizhou.gov.cn/', '地方级', '贵州省', '通用', 'list', 1, 1, 310),
  ('ccgp_henan', '河南省政府采购网', 'http://www.ccgp-henan.gov.cn/', 'http://www.ccgp-henan.gov.cn/', '地方级', '河南省', '通用', 'list', 1, 1, 320),
  ('ccgp_tianjin', '天津市政府采购网', 'http://www.ccgp-tianjin.gov.cn/', 'http://www.ccgp-tianjin.gov.cn/', '地方级', '天津市', '通用', 'list', 1, 1, 330),
  ('ccgp_jiangsu', '江苏政府采购网', 'http://www.ccgp-jiangsu.gov.cn/', 'http://www.ccgp-jiangsu.gov.cn/', '地方级', '江苏省', '通用', 'list', 1, 1, 340),
  ('ccgp_hebei', '河北省政府采购网', 'http://www.ccgp-hebei.gov.cn/', 'http://www.ccgp-hebei.gov.cn/', '地方级', '河北省', '通用', 'list', 1, 1, 350),
  ('ccgp_xizang', '西藏采购网', 'http://www.ccgp-xizang.gov.cn/', 'http://www.ccgp-xizang.gov.cn/', '地方级', '西藏自治区', '通用', 'list', 1, 1, 360),
  ('ccgp_gansu', '甘肃政府采购网', 'http://www.ccgp-gansu.gov.cn/', 'http://www.ccgp-gansu.gov.cn/', '地方级', '甘肃省', '通用', 'list', 1, 1, 370),
  ('ccgp_shaanxi', '陕西省政府采购网', 'http://www.ccgp-shaanxi.gov.cn/', 'http://www.ccgp-shaanxi.gov.cn/', '地方级', '陕西省', '通用', 'list', 1, 1, 380),
  ('ccgp_hunan', '湖南省政府采购网', 'http://www.ccgp-hunan.gov.cn/', 'http://www.ccgp-hunan.gov.cn/', '地方级', '湖南省', '通用', 'list', 1, 1, 390);

-- ============================================================
-- 9. 订阅匹配任务（任务账本 + 固定情报范围）
--
-- 背景：匹配从打标流程里解耦为独立队列任务。每个任务携带固定情报范围，
-- 任务之间互不影响；账本是管理端“订阅匹配”作业面的唯一事实来源
-- （Redis 里的队列任务几天后就过期，撑不起可检索、可删除的列表）。
-- ============================================================
CREATE TABLE IF NOT EXISTS `tender_intel_match_task` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '主键',
  `task_no` varchar(64) COLLATE utf8mb4_bin NOT NULL COMMENT '任务号，业务唯一标识',
  `task_type` varchar(24) COLLATE utf8mb4_bin NOT NULL DEFAULT 'collect_run' COMMENT '任务类型：collect_run采集后匹配/manual_notice手工发布/import_batch批量导入/subscription_backfill订阅回溯/manual_rescan管理补扫',
  `scope_kind` varchar(16) COLLATE utf8mb4_bin NOT NULL DEFAULT 'run' COMMENT '范围类型：run采集批次/notice单条情报/batch导入批次/subscription订阅/window时间窗',
  `scope_ref` varchar(128) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '范围标识（run_id / notice_id / import_batch / subscription_id）',
  `range_from` datetime DEFAULT NULL COMMENT '时间窗起（回溯与补扫用）',
  `range_to` datetime DEFAULT NULL COMMENT '时间窗止',
  `notice_id_snapshot` bigint NOT NULL DEFAULT '0' COMMENT '创建时的情报 ID 上界，保证范围固定',
  `cursor_notice_id` bigint NOT NULL DEFAULT '0' COMMENT '断点：已处理到的情报 ID 边界',
  `notice_total` int NOT NULL DEFAULT '0' COMMENT '本次覆盖情报数',
  `scanned_count` int NOT NULL DEFAULT '0' COMMENT '已扫描情报数',
  `matched_notice_count` int NOT NULL DEFAULT '0' COMMENT '命中的情报条数',
  `alert_count` int NOT NULL DEFAULT '0' COMMENT '生成的提醒条数',
  `status` varchar(16) COLLATE utf8mb4_bin NOT NULL DEFAULT 'pending' COMMENT '状态：pending排队中/running执行中/success成功/failed失败/cancelled已取消',
  `priority` varchar(8) COLLATE utf8mb4_bin NOT NULL DEFAULT 'normal' COMMENT '优先级：normal普通/high优先',
  `cancel_requested` tinyint NOT NULL DEFAULT '0' COMMENT '是否请求取消：1=已请求（运行中任务在批次边界协作式退出）',
  `attempts` int NOT NULL DEFAULT '0' COMMENT '已执行次数',
  `retry_of` varchar(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '重试来源任务号（非重试为空）',
  `user_id` bigint NOT NULL DEFAULT '0' COMMENT '关联用户（订阅回溯）',
  `subscription_id` bigint NOT NULL DEFAULT '0' COMMENT '关联订阅（订阅回溯）',
  `queued_task_id` varchar(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '队列任务 ID',
  `last_error` varchar(512) COLLATE utf8mb4_bin NOT NULL DEFAULT '' COMMENT '最近一次失败原因',
  `started_at` datetime DEFAULT NULL COMMENT '开始执行时间',
  `finished_at` datetime DEFAULT NULL COMMENT '结束时间',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_task_no` (`task_no`),
  KEY `idx_status_priority_id` (`status`,`priority`,`id`),
  KEY `idx_subscription` (`subscription_id`,`id`),
  KEY `idx_scope` (`scope_kind`,`scope_ref`),
  KEY `idx_type_created` (`task_type`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin COMMENT='招标情报订阅匹配任务表';

-- 情报记录首个入库批次，把“本次采集新增的这批情报”变成可直接查询的集合
SET @ddl := (SELECT IF(COUNT(*) = 0,
  'ALTER TABLE `tender_intel_notice` ADD COLUMN `collect_run_id` varchar(64) COLLATE utf8mb4_bin NOT NULL DEFAULT '''' COMMENT ''首次入库的采集批次 ID'' AFTER `import_batch`',
  'DO 0')
  FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'tender_intel_notice' AND column_name = 'collect_run_id');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ddl := (SELECT IF(COUNT(*) = 0,
  'ALTER TABLE `tender_intel_notice` ADD KEY `idx_collect_run` (`collect_run_id`,`id`)',
  'DO 0')
  FROM information_schema.statistics
  WHERE table_schema = DATABASE() AND table_name = 'tender_intel_notice' AND index_name = 'idx_collect_run');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 订阅回溯的断点与完成标记（决定“首次启用才回溯”，并支持续跑）
SET @ddl := (SELECT IF(COUNT(*) = 0,
  'ALTER TABLE `tender_intel_subscription` ADD COLUMN `backfill_status` varchar(16) COLLATE utf8mb4_bin NOT NULL DEFAULT ''none'' COMMENT ''回溯状态：none/running/success/failed/cancelled'' AFTER `matched_count`',
  'DO 0')
  FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'tender_intel_subscription' AND column_name = 'backfill_status');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ddl := (SELECT IF(COUNT(*) = 0,
  'ALTER TABLE `tender_intel_subscription` ADD COLUMN `backfill_cursor_notice_id` bigint NOT NULL DEFAULT ''0'' COMMENT ''回溯断点：已回溯到的情报 ID 上界'' AFTER `backfill_status`',
  'DO 0')
  FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'tender_intel_subscription' AND column_name = 'backfill_cursor_notice_id');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @ddl := (SELECT IF(COUNT(*) = 0,
  'ALTER TABLE `tender_intel_subscription` ADD COLUMN `backfilled_at` datetime DEFAULT NULL COMMENT ''回溯完成时间（为空表示从未回溯过）'' AFTER `backfill_cursor_notice_id`',
  'DO 0')
  FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'tender_intel_subscription' AND column_name = 'backfilled_at');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 提醒来源只落库供运维排查，不在用户界面展示
SET @ddl := (SELECT IF(COUNT(*) = 0,
  'ALTER TABLE `tender_intel_alert` ADD COLUMN `match_source` varchar(16) COLLATE utf8mb4_bin NOT NULL DEFAULT ''collect'' COMMENT ''提醒来源：collect采集命中/backfill历史回溯命中'' AFTER `matched_reason`',
  'DO 0')
  FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'tender_intel_alert' AND column_name = 'match_source');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- 采集批次展示本轮匹配出的提醒数
SET @ddl := (SELECT IF(COUNT(*) = 0,
  'ALTER TABLE `tender_intel_collect_run` ADD COLUMN `matched_count` int NOT NULL DEFAULT ''0'' COMMENT ''本轮匹配出的提醒数'' AFTER `enriched_count`',
  'DO 0')
  FROM information_schema.columns
  WHERE table_schema = DATABASE() AND table_name = 'tender_intel_collect_run' AND column_name = 'matched_count');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
