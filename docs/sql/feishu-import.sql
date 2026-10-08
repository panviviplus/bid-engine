-- Run once against smart-bid. No foreign keys: relationships are maintained by the application.
CREATE TABLE IF NOT EXISTS feishu_account_binding (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  app_id VARCHAR(64) NOT NULL,
  open_id VARCHAR(128) NOT NULL,
  tenant_key VARCHAR(64) NOT NULL DEFAULT '',
  user_id BIGINT NOT NULL,
  notify_enabled TINYINT NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  UNIQUE KEY uk_feishu_app_open (app_id, open_id),
  UNIQUE KEY uk_feishu_app_user (app_id, user_id),
  KEY idx_feishu_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS user_extra_profile (
  user_id BIGINT NOT NULL PRIMARY KEY,
  contact_mobile VARCHAR(32) NOT NULL DEFAULT '',
  company_display_name VARCHAR(255) NOT NULL DEFAULT '',
  updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

ALTER TABLE tender_intel_alert
  ADD COLUMN feishu_push_status VARCHAR(20) NOT NULL DEFAULT 'legacy',
  ADD COLUMN feishu_push_attempts INT NOT NULL DEFAULT 0,
  ADD COLUMN feishu_push_next_at DATETIME(3) NULL,
  ADD COLUMN feishu_push_message_id VARCHAR(128) NOT NULL DEFAULT '',
  ADD COLUMN feishu_push_last_error VARCHAR(255) NOT NULL DEFAULT '',
  ADD COLUMN feishu_push_sent_at DATETIME(3) NULL,
  ADD KEY idx_feishu_push_due (feishu_push_status, feishu_push_next_at, id);
