#!/bin/bash

# Configuration
URL="https://coopapi-8nh9o8ol6-gonuts-projects-cffdd0d3.vercel.app/api/v1/loan"
COLLECTION="loan_tracking"
TEST_ID="test_vercel_$(date +%s)"

echo "Testing Vercel Deployment at $URL"
echo "Test ID: $TEST_ID"

# 1. CREATE
echo "\n[1/4] Testing Create..."
CREATE_RES=$(curl -s -X POST "$URL/create" \
  -H "Content-Type: application/json" \
  -d "{
    \"collection\": \"$COLLECTION\",
    \"data\": {
      \"test_id\": \"$TEST_ID\",
      \"action\": \"deployment_test\",
      \"status\": \"initial\"
    }
  }")
echo $CREATE_RES

if [[ $CREATE_RES == *"success"* ]]; then
  echo "✅ Create Passed"
else
  echo "❌ Create Failed"
  exit 1
fi

sleep 1

# 2. GET
echo "\n[2/4] Testing Get..."
GET_RES=$(curl -s -X POST "$URL/get" \
  -H "Content-Type: application/json" \
  -d "{
    \"collection\": \"$COLLECTION\",
    \"filter\": {
      \"test_id\": \"$TEST_ID\"
    }
  }")
echo $GET_RES

if [[ $GET_RES == *"$TEST_ID"* ]]; then
  echo "✅ Get Passed"
else
  echo "❌ Get Failed"
  exit 1
fi

sleep 1

# 3. UPDATE
echo "\n[3/4] Testing Update..."
UPDATE_RES=$(curl -s -X POST "$URL/update" \
  -H "Content-Type: application/json" \
  -d "{
    \"collection\": \"$COLLECTION\",
    \"filter\": {
      \"test_id\": \"$TEST_ID\"
    },
    \"data\": {
      \"status\": \"updated\"
    }
  }")
echo $UPDATE_RES

if [[ $UPDATE_RES == *"success"* ]]; then
  echo "✅ Update Passed"
else
  echo "❌ Update Failed"
  exit 1
fi

sleep 1

# 4. DELETE
echo "\n[4/4] Testing Delete..."
DELETE_RES=$(curl -s -X POST "$URL/delete" \
  -H "Content-Type: application/json" \
  -d "{
    \"collection\": \"$COLLECTION\",
    \"filter\": {
      \"test_id\": \"$TEST_ID\"
    }
  }")
echo $DELETE_RES

if [[ $DELETE_RES == *"success"* ]]; then
  echo "✅ Delete Passed"
else
  echo "❌ Delete Failed"
  exit 1
fi

echo "\n🎉 All tests passed successfully!"
