# Solution: Remove Clerk & Implement Dual-Token Session Architecture

## Overview
We removed the external Clerk login dependency from the local desktop app and the manual Supabase credential setup screen from the teacher dashboard. In their place, we implemented a secure, database-driven **Dual-Token Session Architecture** managed entirely using PostgreSQL logic, Row Level Security (RLS) policies, and a session table in Supabase.

---

## Architectural Details

### 1. Database Session & Account Schema (`supabase_schema.sql`)
We created two tables in Supabase to track accounts and active sessions:
```sql
CREATE TABLE IF NOT EXISTS public.user_accounts (
    username TEXT PRIMARY KEY,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL, -- 'teacher' or 'student'
    classroom_code TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT timezone('utc'::text, now()) NOT NULL
);

CREATE TABLE IF NOT EXISTS public.active_sessions (
    session_token UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_id TEXT NOT NULL,
    role TEXT NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT timezone('utc'::text, now()) NOT NULL
);
```

### 2. Authentication RPC (`login_user`)
We implemented a secure RPC function to verify credentials and generate a UUID session token with role-based lifetimes:
- **Student Desktop Client**: Expires in **10 years** (one-time frictionless login).
- **Teacher Web Dashboard**: Expires in **24 hours** (volatile browser memory session).
```sql
CREATE OR REPLACE FUNCTION login_user(
  p_username TEXT,
  p_password TEXT,
  p_is_desktop BOOLEAN
) RETURNS JSONB AS $$
DECLARE
  v_role TEXT;
  v_class_code TEXT;
  v_token UUID;
  v_expires TIMESTAMP WITH TIME ZONE;
BEGIN
  SELECT role, classroom_code INTO v_role, v_class_code
  FROM public.user_accounts
  WHERE LOWER(username) = LOWER(p_username)
    AND password_hash = crypt(p_password, password_hash);

  IF NOT FOUND THEN
    RAISE EXCEPTION 'Invalid username or password';
  END IF;

  IF p_is_desktop THEN
    v_expires := now() + interval '10 years';
  ELSE
    v_expires := now() + interval '24 hours';
  END IF;

  v_token := gen_random_uuid();

  INSERT INTO public.active_sessions (session_token, entity_id, role, expires_at)
  VALUES (v_token, LOWER(p_username), v_role, v_expires);

  RETURN jsonb_build_object(
    'session_token', v_token,
    'role', v_role,
    'classroom_code', v_class_code,
    'username', LOWER(p_username)
  );
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;
```

### 3. Row Level Security (RLS) Policies
Enabled RLS on student notebooks, logs, and assignments, enforcing checks based on the session token passed inside a custom `x-session-token` HTTP header:
```sql
CREATE POLICY "Allow teachers to view student notebooks in their classroom" ON public.student_notebooks
    FOR SELECT
    USING (
      EXISTS (
        SELECT 1 FROM public.active_sessions s
        JOIN public.user_accounts u ON LOWER(u.username) = LOWER(s.entity_id)
        WHERE s.session_token = get_current_session_token()
          AND s.expires_at > now()
          AND s.role = 'teacher'
          AND LOWER(u.classroom_code) = LOWER(student_notebooks.classroom_code)
      )
    );
```

### 4. Client Sync Headers Update (`internal/study/sync.go`)
Modified the Go client to send the session token in the `Authorization: Bearer <token>` header, while resolving the Supabase Anon Key from environment variables to use in the Kong routing `apikey` header:
```go
req.Header.Set("Content-Type", "application/json")
if apiToken != "" {
    req.Header.Set("Authorization", "Bearer "+apiToken)
}
anonKey := os.Getenv("CLOUD_API_TOKEN")
if anonKey == "" {
    anonKey = os.Getenv("SUPABASE_ANON_KEY")
}
if anonKey != "" {
    req.Header.Set("apikey", anonKey)
}
```

### 5. Native Student Login (`app_settings.go` & `Settings.vue`)
- Removed the old browser-redirection Clerk sign-in logic.
- Redesigned the "Account & Cloud" settings panel to display a simple Student Login Form (Username, Password, Classroom Code) when signed out.
- When signed in, displays a status dashboard displaying `"Cloud Sync Active"`, the username, and the classroom code.
- Added `LoginStudent` and `LogoutStudent` Go backend methods to handle the authorization RPC and store the returned session token and classroom code permanently inside the SQLite `user_settings` table.

### 6. Standalone Teacher Login (`cloud-dashboard/src/App.vue`)
- Replaced the manual Supabase URL and Key entry fields with a premium Email/Password login page.
- Reads connection credentials from compiled-in environment variables.
- Stores the returned teacher session token in `sessionStorage` (cleared automatically on tab close).
- Attaches the custom `x-session-token` header to all REST select, insert, and delete queries.

---

## Verification Plan

### 1. Automated Tests
All packages compile cleanly. Unit tests pass successfully:
```powershell
go test ./...
```

### 2. Manual Verification
- **Web Dashboard**: Sign in via `teacher1` and verify that the app connects to Supabase, pulls data correctly, and logs out when the tab is closed.
- **Desktop Client**: Go to **Settings** -> **Account & Cloud**, submit student credentials, verify that the login persists, and that "Sync with Cloud Now" synchronizes notebooks and logs silently.
