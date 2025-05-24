#!/bin/bash

# API 测试脚本

BASE_URL="http://localhost:8080"
AIGATEWAY_URL="http://localhost:1002"

echo "=== Quota Manager API 测试 ==="

# 1. 健康检查
echo "1. 健康检查..."
curl -s "$BASE_URL/health" | jq .
echo ""

# 2. 获取策略列表
echo "2. 获取策略列表..."
curl -s "$BASE_URL/api/v1/strategies" | jq .
echo ""

# 3. 创建测试策略
echo "3. 创建测试策略..."
curl -s -X POST "$BASE_URL/api/v1/strategies" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test-api-strategy",
    "title": "API测试策略",
    "type": "single",
    "amount": 100,
    "model": "gpt-3.5-turbo",
    "condition": "github-star(\"zgsm\")"
  }' | jq .
echo ""

# 4. 手动触发策略扫描
echo "4. 手动触发策略扫描..."
curl -s -X POST "$BASE_URL/api/v1/strategies/scan" | jq .
echo ""

# 5. 等待一下让策略执行
echo "5. 等待策略执行..."
sleep 3

# 6. 查询用户配额（测试 AiGateway）
echo "6. 查询用户配额..."
curl -s "$AIGATEWAY_URL/v1/chat/completions/quota?consumer=user001" \
  -H "Authorization: Bearer credential3" | jq .
echo ""

echo "=== 测试完成 ==="