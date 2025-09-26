-- Database field addition migration script
-- Execution date: 2025-09-11
-- Description: Add expiry_days field to quota_strategy table

-- Connect to quota_manager database
\c quota_manager;

-- Check if expiry_days field already exists, add if not exists
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public'
        AND table_name = 'quota_strategy'
        AND column_name = 'expiry_days'
    ) THEN
        ALTER TABLE quota_strategy ADD COLUMN expiry_days INTEGER;
        RAISE NOTICE 'Successfully added expiry_days field to quota_strategy table';
    ELSE
        RAISE NOTICE 'expiry_days field already exists in quota_strategy table, skipping addition';
    END IF;
END $$;

-- Add field comment
COMMENT ON COLUMN quota_strategy.expiry_days IS 'Valid days, can be null, used to calculate quota expiration time';
