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
```json
{
    "collection": "loan_applications",
    "data": {
        "memberid": "MEM001",
        "requestamount": 50000,
        "interestrate": 15.0,
        "requestterm": 24
    }
}
```

### 2. Get Data
**POST** `/api/v1/loan/get`
```json
{
    "collection": "loan_applications",
    "filter": {
        "memberid": "MEM001"
    }
}
```

### 3. Update Data
**POST** `/api/v1/loan/update`
```json
{
    "collection": "loan_applications",
    "filter": { "applicationid": "..." },
    "data": { "status": "APPROVED" }
}
```

### 4. Delete Data
**POST** `/api/v1/loan/delete`
```json
{
    "collection": "loan_applications",
    "filter": { "applicationid": "..." }
}
```

## Testing

รันสคริปต์ทดสอบความปลอดภัย (Validation Check):
```bash
./advanced_verification.sh
```