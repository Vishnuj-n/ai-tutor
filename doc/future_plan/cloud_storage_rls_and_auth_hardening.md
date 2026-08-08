# Future Plan: Cloud Storage RLS & Dashboard Auth Hardening

**Module:** Cloud Dashboard (`useDashboard.js`, `Assignments.vue`), Supabase Schema (`04_rls_and_storage.sql`, `storage.objects`)

---

## 1. Current State & Immediate Architecture

During active development and standalone dashboard mode (when the Go backend API server is offline), the Cloud Dashboard relies on direct Supabase REST queries using the public Anon Key (`VITE_SUPABASE_ANON_KEY`).

### Storage Upload Policy (Dev Mode)
To allow seamless PDF uploads from the standalone web dashboard without requiring a running Go auth server or populated `public.active_sessions` database table, the storage RLS policy on `storage.objects` for the `assignments` bucket is configured to permissive bucket matching:

```sql
DROP POLICY IF EXISTS "Teacher PDF Upload Policy" ON storage.objects;
DROP POLICY IF EXISTS "Allow Public Upload to Assignments Bucket" ON storage.objects;

CREATE POLICY "Allow Public Upload to Assignments Bucket" ON storage.objects FOR INSERT TO public
WITH CHECK (bucket_id = 'assignments');
```

### Protection in Current State
Even with this simplified RLS policy, Supabase Storage's bucket-level definitions enforce the following security boundaries:
- **MIME Type Enforcement**: Restricted exclusively to `application/pdf`.
- **Max File Size**: Capped at 50MB per uploaded file.

---

## 2. Future Production Hardening Plan

Before deploying the Cloud Dashboard to a multi-tenant public production environment, the storage RLS policies and authentication flows should be hardened to prevent unauthorized file uploads by arbitrary clients possessing the anon API key.

### Planned Tasks for Production Hardening

#### A. Session-Backed Fallback Login
Update `loginTeacher()` fallback logic in `cloud-dashboard/src/composables/useDashboard.js` to call the Supabase PostgreSQL RPC function `rpc/login_user` instead of directly selecting from `user_accounts`.
- Calling `login_user` creates a valid session UUID entry in `public.active_sessions`.
- The returned `session_token` will be passed in the `x-session-token` HTTP header on all subsequent REST and Storage API requests.

#### B. Restoring Strict Teacher Session Storage Policy
Re-enable strict session-based RLS on `storage.objects` in `cloud-dashboard/supabase/04_rls_and_storage.sql`:

```sql
DROP POLICY IF EXISTS "Allow Public Upload to Assignments Bucket" ON storage.objects;
DROP POLICY IF EXISTS "Teacher PDF Upload Policy" ON storage.objects;

CREATE POLICY "Teacher PDF Upload Policy" ON storage.objects FOR INSERT TO public
WITH CHECK (
  bucket_id = 'assignments'
  AND LOWER(storage.extension(name)) = 'pdf'
  AND EXISTS (
    SELECT 1 FROM public.active_sessions s
    WHERE s.session_token = get_current_session_token()
      AND s.role = 'teacher'
      AND s.expires_at > now()
  )
);
```

#### C. Presigned Backend Upload URLs (Optional Enterprise Upgrade)
Alternatively, implement a backend route on the Go server (`POST /api/assignments/upload-url`) using the Supabase `service_role` key to issue short-lived presigned upload URLs directly to validated teacher clients, allowing the `assignments` storage bucket to remain completely private with zero public `INSERT` permissions.
