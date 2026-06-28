# Solution: Supabase Cloud Sync & Standalone Teacher Dashboard

## Overview
We replaced the need for a custom Express backend by implementing a "backend-less" cloud sync architecture using Supabase. A single unified PostgreSQL RPC function (`handle_cloud_sync`) manages transactional progress upserts and pushes teacher assignments to student clients. We also created a standalone Vue 3 / Vite web dashboard for teachers to monitor student pacing, track Socratic rescue alerts, and issue new assignments.

---

## Architectural Details

### 1. Client HTTP Header Update
To support Supabase Kong gateway routing, the Go client sync code was modified to include the `apikey` header alongside the Bearer token:
- **File**: `internal/study/sync.go`
- **Code**:
  ```go
  req.Header.Set("Content-Type", "application/json")
  if apiToken != "" {
      req.Header.Set("Authorization", "Bearer "+apiToken)
      req.Header.Set("apikey", apiToken)
  }
  ```

### 2. Supabase SQL Database Schema
- **File**: `supabase_schema.sql`
- **Tables**:
  - `student_notebooks`: Tracks study statuses (`dormant`/`active`/`completed`) and Socratic rescue alerts (`external_help_required`).
  - `student_review_logs`: Stores immutable historical spaced repetition logs (rating, scheduled interval, page number, and hash).
  - `teacher_assignments`: Holds PDF files published by teachers for student auto-downloading.
- **RPC Endpoint (`handle_cloud_sync`)**:
  Performs bulk upserts of student notebooks, writes new review logs, and returns assignments matching the student's classroom code.

### 3. Standalone Teacher Dashboard
- **Location**: `/cloud-dashboard`
- **Tech Stack**: Vue 3 + Vite. Uses raw `fetch` calls to Supabase REST and RPC endpoints, requiring zero external client libraries.
- **Features**:
  - Credential management stored in `localStorage`.
  - Statistics overview (enrolled count, review logs count, recall pass rate percentage, and red alerts count).
  - Glowing indicator highlight for Socratic remediation failure alerts.
  - Accordion folder details for individual student study paths.
  - Publish form to assign direct PDF download links to the classroom.

---

## How to Run & Verify

### 1. Supabase Initialization
1. Spin up a new Supabase project.
2. Open the **SQL Editor** in Supabase, paste the contents of `supabase_schema.sql`, and run the query to build the schema and RPC function.
3. Fetch your **Project API URL** and **Anon Key** from Project Settings -> API.

### 2. Run Local Desktop Client
1. Start the client:
   ```powershell
   wails dev
   ```
2. Navigate to **Settings** -> **Account & Cloud**.
3. Configure the **Sync Server URL** (e.g. `https://<id>.supabase.co/rest/v1/rpc/handle_cloud_sync`), the **Access Token** (`<anon-key>`), and a **Classroom Code** (e.g. `BIO101`).
4. Click **"Sync with Cloud Now"**.

### 3. Run Teacher Dashboard
1. Navigate to the dashboard workspace and start the Vite server:
   ```powershell
   cd cloud-dashboard
   npm run dev
   ```
2. Open your browser to `http://localhost:5173`.
3. Input your Supabase credentials and classroom code to load classroom data.
