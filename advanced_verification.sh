#!/bin/bash

BASE_URL="http://localhost:8080"

echo "1. Testing Allowed Collection (Should Succeed)..."
curl -X POST "$BASE_URL/api/v1/loan/create" \
  -H "Content-Type: application/json" \
  -d '{
    "collection": "loan_applications",
    "data": {"test": "ok"}
  }' | python3 -m json.tool

echo -e "\n\n2. Testing Blocked Collection (Should Fail 403)..."
curl -X POST "$BASE_URL/api/v1/loan/create" \
  -H "Content-Type: application/json" \
  -d '{
    "collection": "blocked_collection",
    "data": {"test": "fail"}
  }' | python3 -m json.tool
