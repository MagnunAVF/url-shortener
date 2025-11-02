CREATE DATABASE IF NOT EXISTS analytics;

CREATE TABLE IF NOT EXISTS analytics.logs
(
    time  DateTime64(3, 'UTC') DEFAULT now64(3),
    level LowCardinality(String),
    msg   String,
    data  JSON
)
ENGINE = MergeTree
ORDER BY (time, level)
SETTINGS index_granularity = 8192;
