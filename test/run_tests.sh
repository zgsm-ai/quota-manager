#!/bin/bash

echo "=== 配额管理器集成测试启动脚本 ==="

# 检查Go是否安装
if ! command -v go &> /dev/null; then
    echo "Go未安装，请先安装Go"
    exit 1
fi

# 检查PostgreSQL是否安装
if ! command -v psql &> /dev/null; then
    echo "PostgreSQL未安装，请先安装PostgreSQL"
    exit 1
fi

# 设置环境变量
export POSTGRES_HOST=${POSTGRES_HOST:-localhost}
export POSTGRES_PORT=${POSTGRES_PORT:-5432}
export POSTGRES_USER=${POSTGRES_USER:-postgres}
export POSTGRES_PASSWORD=${POSTGRES_PASSWORD:-password}
export POSTGRES_DB=${POSTGRES_DB:-quota_manager}

echo "使用的数据库配置："
echo "  主机: $POSTGRES_HOST"
echo "  端口: $POSTGRES_PORT"
echo "  用户: $POSTGRES_USER"
echo "  数据库: $POSTGRES_DB"

# 进入测试目录
cd "$(dirname "$0")"

# 检查数据库连接
echo "检查数据库连接..."
PGPASSWORD=$POSTGRES_PASSWORD psql -h $POSTGRES_HOST -p $POSTGRES_PORT -U $POSTGRES_USER -d $POSTGRES_DB -c "SELECT 1;" > /dev/null 2>&1
if [ $? -ne 0 ]; then
    echo "数据库连接失败，请检查数据库是否运行并且配置正确"
    echo "尝试初始化数据库..."

    # 创建数据库（如果不存在）
    PGPASSWORD=$POSTGRES_PASSWORD psql -h $POSTGRES_HOST -p $POSTGRES_PORT -U $POSTGRES_USER -c "CREATE DATABASE $POSTGRES_DB;" 2>/dev/null

    # 初始化数据库表结构
    PGPASSWORD=$POSTGRES_PASSWORD psql -h $POSTGRES_HOST -p $POSTGRES_PORT -U $POSTGRES_USER -d $POSTGRES_DB -f ../scripts/init_db.sql

    if [ $? -ne 0 ]; then
        echo "数据库初始化失败，请检查配置"
        exit 1
    fi
fi

echo "数据库连接成功"

# 更新依赖
echo "更新Go依赖..."
cd ..
go mod tidy

# 运行集成测试
echo "开始运行集成测试..."
cd test
go run integration_main.go

echo "集成测试完成"