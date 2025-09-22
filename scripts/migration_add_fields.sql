-- 数据库字段添加迁移脚本
-- 执行日期: 2025-09-11
-- 说明: 为 quota_strategy 表添加 expiry_days 字段

-- 连接到 quota_manager 数据库
\c quota_manager;

-- 检查 expiry_days 字段是否已存在，如果不存在则添加
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_schema = 'public' 
        AND table_name = 'quota_strategy' 
        AND column_name = 'expiry_days'
    ) THEN
        ALTER TABLE quota_strategy ADD COLUMN expiry_days INTEGER;
        RAISE NOTICE '已成功为 quota_strategy 表添加 expiry_days 字段';
    ELSE
        RAISE NOTICE 'quota_strategy 表的 expiry_days 字段已存在，跳过添加';
    END IF;
END $$;

-- 添加字段注释
COMMENT ON COLUMN quota_strategy.expiry_days IS '有效天数，可为空，用于计算配额过期时间';
