CREATE DATABASE IF NOT EXISTS analytics;

CREATE TABLE IF NOT EXISTS analytics.logs
(
    `time` DateTime64(6),
    `log_level` LowCardinality(String),
    `msg` String,
    `request_id` String,
    `host` String,
    `data` Nullable(String)
)
ENGINE = MergeTree()
ORDER BY (time);