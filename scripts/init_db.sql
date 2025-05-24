CREATE DATABASE quota_manager;

-- 保持原始表结构不变
CREATE TABLE IF NOT EXISTS user_info (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255),
    github_username VARCHAR(255),
    email VARCHAR(255),
    phone VARCHAR(255),
    github_star TEXT,
    vip INTEGER DEFAULT 0,
    org VARCHAR(255),
    register_time TIMESTAMP,
    access_time TIMESTAMP,
    create_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    update_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 保持 SERIAL 类型（不替换为 GENERATED AS IDENTITY）
CREATE TABLE IF NOT EXISTS quota_strategy (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL,
    title VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,
    amount INTEGER NOT NULL,
    model VARCHAR(255) NOT NULL,
    periodic_expr VARCHAR(255),
    condition TEXT,
    create_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    update_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 保持原始外键语法
CREATE TABLE IF NOT EXISTS quota_execute (
    id SERIAL PRIMARY KEY,
    strategy_id INTEGER NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    batch_number VARCHAR(20) NOT NULL,
    status VARCHAR(50) NOT NULL,
    create_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    update_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (strategy_id) REFERENCES quota_strategy(id)
);

-- 保持原始索引语法（PG 9.5+ 支持 IF NOT EXISTS）
CREATE INDEX IF NOT EXISTS idx_quota_execute_strategy_id ON quota_execute(strategy_id);
CREATE INDEX IF NOT EXISTS idx_quota_execute_user_id ON quota_execute(user_id);
CREATE INDEX IF NOT EXISTS idx_quota_execute_batch_number ON quota_execute(batch_number);
