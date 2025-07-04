#!/bin/bash

# API Test Script

BASE_URL="http://localhost:8099"
AIGATEWAY_URL="http://localhost:1002"

# JWT token for authentication (contains universal_id field in correct JWT format)
TEST_TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1bml2ZXJzYWxfaWQiOiJ1c2VyMDAxIiwibmFtZSI6IkpvaG4gRG9lIiwic3RhZmZJRCI6ImVtcDAwMSIsImdpdGh1YiI6ImpvaG5kb2UiLCJwaG9uZSI6IjEzODAwMTM4MDAxIn0.signature"

echo "=== Quota Manager API Test ==="

# 1. Health Check
echo "1. Health Check..."
curl -s "$BASE_URL/quota-manager/health" | jq .
echo ""

# 2. Get All Strategies
echo "2. Get All Strategies..."
curl -s "$BASE_URL/quota-manager/api/v1/strategies" | jq .
echo ""

# 3. Get Enabled Strategies
echo "3. Get Enabled Strategies..."
curl -s "$BASE_URL/quota-manager/api/v1/strategies?status=enabled" | jq .
echo ""

# 4. Get Disabled Strategies
echo "4. Get Disabled Strategies..."
curl -s "$BASE_URL/quota-manager/api/v1/strategies?status=disabled" | jq .
echo ""

# 5. Create Test Strategy
echo "5. Create Test Strategy..."
STRATEGY_RESPONSE=$(curl -s -X POST "$BASE_URL/quota-manager/api/v1/strategies" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test-api-strategy",
    "title": "API Test Strategy",
    "type": "single",
    "amount": 100,
    "model": "gpt-3.5-turbo",
    "condition": "github-star(\"zgsm\")",
    "status": false
  }')
echo "$STRATEGY_RESPONSE" | jq .
STRATEGY_ID=$(echo "$STRATEGY_RESPONSE" | jq -r '.data.id')
echo "Created strategy ID: $STRATEGY_ID"
echo ""

# 6. Get Created Strategy Details
echo "6. Get Created Strategy Details..."
curl -s "$BASE_URL/quota-manager/api/v1/strategies/$STRATEGY_ID" | jq .
echo ""

# 7. Enable Strategy
echo "7. Enable Strategy..."
curl -s -X POST "$BASE_URL/quota-manager/api/v1/strategies/$STRATEGY_ID/enable" | jq .
echo ""

# 8. Verify Strategy Status Update
echo "8. Verify Strategy Status Update..."
curl -s "$BASE_URL/quota-manager/api/v1/strategies/$STRATEGY_ID" | jq .
echo ""

# 9. Manually Trigger Strategy Scan
echo "9. Manually Trigger Strategy Scan..."
curl -s -X POST "$BASE_URL/quota-manager/api/v1/strategies/scan" | jq .
echo ""

# 10. Wait for Strategy Execution
echo "10. Wait for Strategy Execution..."
sleep 3

# 11. Query User Quota (Test AiGateway)
echo "11. Query User Quota..."
curl -s "$AIGATEWAY_URL/v1/chat/completions/quota?user_id=user001" \
  -H "x-admin-key: credential3" | jq .
echo ""

# 12. Disable Strategy
echo "12. Disable Strategy..."
curl -s -X POST "$BASE_URL/quota-manager/api/v1/strategies/$STRATEGY_ID/disable" | jq .
echo ""

# 13. Verify Strategy is Disabled
echo "13. Verify Strategy is Disabled..."
curl -s "$BASE_URL/quota-manager/api/v1/strategies/$STRATEGY_ID" | jq .
echo ""

# 14. Update Strategy (including status)
echo "14. Update Strategy Status..."
curl -s -X PUT "$BASE_URL/quota-manager/api/v1/strategies/$STRATEGY_ID" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "API Test Strategy (Updated)",
    "status": true
  }' | jq .
echo ""

# 15. Verify Update Results
echo "15. Verify Update Results..."
curl -s "$BASE_URL/quota-manager/api/v1/strategies/$STRATEGY_ID" | jq .
echo ""

# 16. Test Boolean String Query
echo "16. Test Boolean String Query..."
echo "16a. Query Enabled Strategies (?status=true):"
curl -s "$BASE_URL/quota-manager/api/v1/strategies?status=true" | jq '.data.total'
echo "16b. Query Disabled Strategies (?status=false):"
curl -s "$BASE_URL/quota-manager/api/v1/strategies?status=false" | jq '.data.total'
echo ""

# 17. Test Quota Management APIs
echo "17. Test Quota Management APIs..."

echo "17a. Get User Quota:"
curl -s "$BASE_URL/quota-manager/api/v1/quota" \
  -H "Authorization: Bearer $TEST_TOKEN" | jq .
echo ""

echo "17b. Get Quota Audit Records:"
curl -s "$BASE_URL/quota-manager/api/v1/quota/audit?page=1&page_size=5" \
  -H "Authorization: Bearer $TEST_TOKEN" | jq .
echo ""

echo "17c. Test Transfer Out (will fail due to insufficient quota, but tests API structure):"
curl -s -X POST "$BASE_URL/quota-manager/api/v1/quota/transfer-out" \
  -H "Authorization: Bearer $TEST_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "receiver_id": "user002",
    "quota_list": [
      {
        "amount": 1,
        "expiry_date": "2025-06-30T23:59:59Z"
      }
    ]
  }' | jq .
echo ""

# 18. Cleanup: Delete Test Strategy
echo "18. Cleanup: Delete Test Strategy..."
curl -s -X DELETE "$BASE_URL/quota-manager/api/v1/strategies/$STRATEGY_ID" | jq .
echo ""

echo "=== Test Completed ==="