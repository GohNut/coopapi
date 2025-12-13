# Dynamic Loan API

API สำหรับระบบสินเชื่อสหกรณ์ (Coop Digital) พัฒนาด้วย Go (Echo Framework) และ MongoDB Atlas รองรับโครงสร้างข้อมูลแบบ Dynamic

## Features

- **Dynamic Gateway Pattern**: รองรับ Data Structure ที่หลากหลายจาก Frontend
- **MongoDB Atlas Integration**: เชื่อมต่อฐานข้อมูล Cloud
- **CRUD Operations**: Create, Get, Update, Delete สำหรับ `loan_applications` และ collections อื่นๆ
- **Validation**:
  - **Collection Whitelist**: ป้องกันการเข้าถึง Collection ที่ไม่อนุญาต
  - **Data Size Limit**: จำกัดขนาด Payload (16MB limit)
- **Automatic Indexing**: สร้าง Index อัตโนมัติเมื่อเริ่มระบบ (Unique constraints, sorting indexes)
- **Calculations**: คำนวณค่างวด (Installment), เงินต้น, และดอกเบี้ยอัตโนมัติ

## Prerequisites

- Go 1.20+
- MongoDB Atlas Cluster

## Setup

1.  **Clone Project**
    ```bash
    git clone <repository_url>
    cd loan-dynamic-api
    ```

2.  **Environment Variables**
    สร้างไฟล์ `.env` :
    ```env
    MONGODB_URI=mongodb+srv://<user>:<password>@<cluster>.mongodb.net/?retryWrites=true&w=majority
    MONGODB_DB=coop_digital
    API_PORT=8080
    CORS_ORIGINS=*
    ```

## Running the API

ใช้คำสั่ง `go run` เพื่อเริ่ม Server:

```bash
go run main.go
```

Server จะทำงานที่ `http://localhost:8080`

## API Endpoints

### 1. Create Loan / Insert Data
**POST** `/api/v1/loan/create`

สร้างคำขอสินเชื่อหรือเพิ่มข้อมูลใหม่ ระบบจะคำนวณค่างวด (installment), เงินต้นรวม และข้อมูลอื่นๆ อัตโนมัติ

**Request Body:**
```json
{
    "collection": "loan_applications",
    "data": {
        "memberid": "MEM001",
        "requestamount": 50000,
        "interestrate": 15.0,
        "requestterm": 24,
        "loantype": "สินเชื่อสามัญ"
    }
}
```

**Response (Success):**
```json
{
    "status": "success",
    "code": 200,
    "inserted_id": "675a1b2c3d4e5f6g7h8i9j0k",
    "data": {
        "applicationid": "REQ-2024-001",
        "memberid": "MEM001",
        "requestamount": 50000,
        "interestrate": 15.0,
        "requestterm": 24,
        "installmentamount": 2400.50,
        "totalamount": 57612.00,
        "createdat": "2024-12-12T23:50:00Z"
    }
}
```

---

### 2. Get Data
**POST** `/api/v1/loan/get`

ดึงข้อมูลสินเชื่อตาม filter ที่กำหนด รองรับ pagination และการจำกัดจำนวนผลลัพธ์

**Request Body:**
```json
{
    "collection": "loan_applications",
    "filter": {
        "memberid": "MEM001",
        "status": "PENDING"
    },
    "limit": 10,
    "skip": 0
}
```

**Response (Success):**
```json
{
    "status": "success",
    "code": 200,
    "count": 2,
    "data": [
        {
            "_id": "675a1b2c3d4e5f6g7h8i9j0k",
            "applicationid": "REQ-2024-001",
            "memberid": "MEM001",
            "status": "PENDING",
            "requestamount": 50000,
            "interestrate": 15.0,
            "requestterm": 24
        }
    ]
}
```

**Query Options:**
- `filter` (object) - เงื่อนไขการค้นหา (MongoDB query format)
- `limit` (int) - จำนวนผลลัพธ์สูงสุด (default: 100)
- `skip` (int) - ข้ามผลลัพธ์กี่รายการ (สำหรับ pagination)

---

### 3. Update Data
**POST** `/api/v1/loan/update`

อัปเดตข้อมูลสินเชื่อตาม filter ที่กำหนด **รองรับ Upsert** (สร้างใหม่ถ้าไม่เจอ)

**Request Body (Update):**
```json
{
    "collection": "loan_applications",
    "filter": { 
        "applicationid": "REQ-2024-001" 
    },
    "data": { 
        "status": "APPROVED",
        "approvedamount": 50000,
        "approvedby": "ADMIN001"
    }
}
```

**Request Body (Upsert):**
```json
{
    "collection": "loan_applications",
    "filter": { 
        "applicationid": "REQ-2024-999" 
    },
    "data": { 
        "memberid": "MEM002",
        "status": "PENDING",
        "requestamount": 100000
    },
    "upsert": true
}
```

**Response (Success - Update):**
```json
{
    "status": "success",
    "code": 200,
    "matched_count": 1,
    "modified_count": 1,
    "upserted_id": null
}
```

**Response (Success - Upsert):**
```json
{
    "status": "success",
    "code": 200,
    "matched_count": 0,
    "modified_count": 0,
    "upserted_id": "675a1b2c3d4e5f6g7h8i9j0k"
}
```

**Upsert Behavior:**
- `"upsert": true` - ถ้าไม่เจอ document ที่ match กับ filter จะสร้างใหม่
- `"upsert": false` หรือไม่ระบุ - จะ update เฉพาะเมื่อเจอ document ที่ match เท่านั้น

---

### 4. Delete Data
**POST** `/api/v1/loan/delete`

ลบข้อมูลสินเชื่อตาม filter ที่กำหนด

**Request Body:**
```json
{
    "collection": "loan_applications",
    "filter": { 
        "applicationid": "REQ-2024-001",
        "status": "REJECTED"
    }
}
```

**Response (Success):**
```json
{
    "status": "success",
    "code": 200,
    "deleted_count": 1
}
```

---

## Error Responses

API จะส่ง Error Response ในรูปแบบต่อไปนี้:

**Validation Error (400):**
```json
{
    "status": "error",
    "code": 400,
    "message": "Invalid request body",
    "error": "json: cannot unmarshal array into Go value of type map[string]interface {}"
}
```

**Forbidden Collection (403):**
```json
{
    "status": "error",
    "code": 403,
    "message": "Collection not allowed"
}
```

**Payload Too Large (413):**
```json
{
    "status": "error",
    "code": 413,
    "message": "Request payload too large (max 16MB)"
}
```

**Database Error (503):**
```json
{
    "status": "error",
    "code": 503,
    "message": "MongoDB Atlas is not connected"
}
```

---

## Validation Rules

### Collection Whitelist
เข้าถึงได้เฉพาะ Collections ต่อไปนี้:
- `loan_applications`
- `loan_products`
- `member_profiles`

### Data Size Limit
- Payload สูงสุด: **16 MB**
- ใช้สำหรับป้องกัน DoS attacks และควบคุมการใช้ทรัพยากร

## Testing

รันสคริปต์ทดสอบความปลอดภัย (Validation Check):
```bash
./advanced_verification.sh
```