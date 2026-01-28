CREATE DATABASE IF NOT EXISTS `mizuflow` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

USE `mizuflow`;

CREATE TABLE IF NOT EXISTS `feature_audits` (
    `id`         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    `env`        VARCHAR(32) NOT NULL DEFAULT 'dev',
    `namespace`  VARCHAR(64) NOT NULL DEFAULT 'default',
    `key`        VARCHAR(128) NOT NULL COMMENT 'key of the feature',
    `old_value`  TEXT COMMENT 'old value',
    `new_value`  TEXT COMMENT 'new value',
    `type`       VARCHAR(32)  COMMENT 'business type: bool, strategy, etc.',
    `operator`   VARCHAR(64)  DEFAULT 'system' COMMENT 'operator ID',
    `trace_id`   VARCHAR(36)  NOT NULL COMMENT 'UUID for full traceability',
    `ip`         VARCHAR(45)  COMMENT 'operator IP address',
    `created_at` TIMESTAMP    DEFAULT CURRENT_TIMESTAMP COMMENT 'timestamp',
    INDEX `idx_key` (`key`),
    INDEX `idx_trace_id` (`trace_id`),
    INDEX `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='MizuFlow feature change audit table';

CREATE TABLE IF NOT EXISTS `feature_master` (
    `id`          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    `env`         VARCHAR(32) NOT NULL DEFAULT 'dev' COMMENT 'environment',
    `namespace`   VARCHAR(64) NOT NULL DEFAULT 'default' COMMENT 'namespace for grouping features',
    `key`         VARCHAR(128) NOT NULL COMMENT 'key of the feature',
    `current_val` TEXT COMMENT 'current effective value',
    `type`        VARCHAR(32) NOT NULL DEFAULT 'string' COMMENT 'type: bool, json, strategy',
    `version`     BIGINT UNSIGNED NOT NULL DEFAULT 1 COMMENT 'logical version number, incremented with each change',
    `description` VARCHAR(255) COMMENT 'description of the feature for human understanding',
    `status`      TINYINT DEFAULT 1 COMMENT '1: enabled, 0: archived/disabled',
    `created_at`  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    `updated_at`  TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE INDEX `idx_key_env_ns` (`namespace`, `env`, `key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='MizuFlow feature master table';


CREATE TABLE IF NOT EXISTS `outbox_events` (
    `id`          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    `key`         VARCHAR(128) NOT NULL,
    `payload`     TEXT NOT NULL COMMENT 'JSON to be sent to etcd',
    `status`      TINYINT NOT NULL DEFAULT 0 COMMENT '0: pending, 1: completed, 2: permanently failed',
    `retry_count` INT NOT NULL DEFAULT 0,
    `trace_id`    VARCHAR(36) NOT NULL,
    `created_at`  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    `updated_at`  TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX `idx_status_retry` (`status`, `retry_count`),
    INDEX `idx_key` (`key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4; COMMENT='MizuFlow outbox event table for reliable event publishing';


CREATE TABLE IF NOT EXISTS `sdk_clients` (
    `id`          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    `app_id`     VARCHAR(64) NOT NULL COMMENT 'SDK client application',
    `api_key` VARCHAR(64) NOT NULL COMMENT 'SDK client key for authentication',
    `env`        VARCHAR(32) NOT NULL DEFAULT 'dev' COMMENT 'environment',
    `status`     TINYINT NOT NULL DEFAULT 1 COMMENT '1: active, 0: inactive',
    `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE INDEX `idx_api_key_env` (`api_key`, `env`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='MizuFlow SDK clients table';