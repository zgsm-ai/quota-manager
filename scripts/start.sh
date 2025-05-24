#!/bin/bash

# 启动脚本

echo "Starting Quota Manager System..."

# 检查是否安装了 Go
if ! command -v go &> /dev/null; then
    echo "Go is not installed. Please install Go first."
    exit 1
fi

# 检查是否安装了 PostgreSQL
if ! command -v psql &> /dev/null; then
    echo "PostgreSQL is not installed. Please install PostgreSQL first."
    exit 1
fi

# 设置环境变量
export POSTGRES_HOST=${POSTGRES_HOST:-localhost}
export POSTGRES_PORT=${POSTGRES_PORT:-5432}
export POSTGRES_USER=${POSTGRES_USER:-postgres}
export POSTGRES_PASSWORD=${POSTGRES_PASSWORD:-password}
export POSTGRES_DB=${POSTGRES_DB:-quota_manager}

echo "Environment variables set:"
echo "  POSTGRES_HOST: $POSTGRES_HOST"
echo "  POSTGRES_PORT: $POSTGRES_PORT"
echo "  POSTGRES_USER: $POSTGRES_USER"
echo "  POSTGRES_DB: $POSTGRES_DB"

# 进入项目目录
cd "$(dirname "$0")/.."

# 下载依赖
echo "Downloading dependencies..."
go mod tidy

# 初始化数据库（如果需要）
echo "Initializing database..."
if [ -f "scripts/init_db.sql" ]; then
    PGPASSWORD=$POSTGRES_PASSWORD psql -h $POSTGRES_HOST -p $POSTGRES_PORT -U $POSTGRES_USER -f scripts/init_db.sql
fi

# 生成测试数据
echo "Generating test data..."
cd scripts
go run generate_data.go
cd ..

# 启动 AiGateway 模拟服务
echo "Starting AiGateway mock service..."
cd ../aigateway-mock
go mod tidy
nohup go run main.go > aigateway.log 2>&1 &
AIGATEWAY_PID=$!
echo "AiGateway mock service started with PID: $AIGATEWAY_PID"
cd ../quota-manager

# 启动主服务
echo "Starting Quota Manager service..."
go run cmd/main.go

# 清理
echo "Shutting down services..."
if [ ! -z "$AIGATEWAY_PID" ]; then
    kill $AIGATEWAY_PID
    echo "AiGateway mock service stopped"
fi