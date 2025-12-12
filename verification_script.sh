#!/bin/bash

BASE_URL="http://localhost:8080"

echo "1. Checking Health..."
curl -s "$BASE_URL/health" | python3 -m json.tool

echo -e "\n\n2. Creating Loan Application..."
CREATE_PAYLOAD='{
    "collection": "loan_applications",
    "data": {
        "memberid": "TEST_MEM001",
        "productname": "Test Loan",
        "requestamount": 100000,
        "requestterm": 12,
        "interestrate": 5.0,
        "status": "pending"
    },
    "upsert": true
}'
curl -X POST "$BASE_URL/api/v1/loan/create" \
  -H "Content-Type: application/json" \
  -d "$CREATE_PAYLOAD" | python3 -m json.tool

echo -e "\n\n3. Getting Loan Application..."
GET_PAYLOAD='{
    "collection": "loan_applications",
    "filter": {
        "memberid": "TEST_MEM001"
    },
    "limit": 1
}'
curl -X POST "$BASE_URL/api/v1/loan/get" \
  -H "Content-Type: application/json" \
  -d "$GET_PAYLOAD" | python3 -m json.tool
