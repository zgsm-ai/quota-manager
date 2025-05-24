CREATE DATABASE quota_manager;

-- Keep original table structure unchanged
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

-- Keep SERIAL type (do not replace with GENERATED AS IDENTITY)
CREATE TABLE IF NOT EXISTS quota_strategy (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL,
    title VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,
    amount INTEGER NOT NULL,
    model VARCHAR(255) NOT NULL,
    periodic_expr VARCHAR(255),
    condition TEXT,
    status BOOLEAN DEFAULT true NOT NULL,  -- Status field: true=enabled, false=disabled
    create_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    update_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Keep original foreign key syntax
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

-- Keep original index syntax (PG 9.5+ supports IF NOT EXISTS)
CREATE INDEX IF NOT EXISTS idx_quota_execute_strategy_id ON quota_execute(strategy_id);
CREATE INDEX IF NOT EXISTS idx_quota_execute_user_id ON quota_execute(user_id);
CREATE INDEX IF NOT EXISTS idx_quota_execute_batch_number ON quota_execute(batch_number);

-- Add index for strategy status field to improve query performance
CREATE INDEX IF NOT EXISTS idx_quota_strategy_status ON quota_strategy(status);
