-- Supabase Schema for AI Tutor Cloud Sync
-- Place this script into the Supabase SQL Editor and run it.

-- Enable UUID extension if not enabled
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 1. Student Notebooks Table
CREATE TABLE IF NOT EXISTS student_notebooks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    student_token TEXT NOT NULL,
    classroom_code TEXT NOT NULL,
    file_hash TEXT NOT NULL,
    filename TEXT NOT NULL,
    title TEXT NOT NULL,
    study_status TEXT NOT NULL,
    external_help_required BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT timezone('utc'::text, now()) NOT NULL,
    UNIQUE (student_token, file_hash)
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_student_notebooks_classroom ON student_notebooks(classroom_code);
CREATE INDEX IF NOT EXISTS idx_student_notebooks_student ON student_notebooks(student_token);

-- Disable Row Level Security (RLS) for prototype simplicity
ALTER TABLE student_notebooks DISABLE ROW LEVEL SECURITY;


-- 2. Student Review Logs Table (immutable spaced repetition records)
CREATE TABLE IF NOT EXISTS student_review_logs (
    id TEXT PRIMARY KEY, -- Log UUID sent from local app
    student_token TEXT NOT NULL,
    classroom_code TEXT NOT NULL,
    file_hash TEXT NOT NULL,
    page_number INTEGER NOT NULL,
    activity_type TEXT NOT NULL,
    reference_id TEXT NOT NULL,
    reviewed_at BIGINT NOT NULL, -- Unix timestamp in seconds
    rating INTEGER NOT NULL,
    scheduled_days INTEGER NOT NULL,
    state_before_json TEXT,
    state_after_json TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT timezone('utc'::text, now()) NOT NULL
);

-- Indexes for metrics calculations
CREATE INDEX IF NOT EXISTS idx_student_review_logs_classroom ON student_review_logs(classroom_code);
CREATE INDEX IF NOT EXISTS idx_student_review_logs_student ON student_review_logs(student_token);
CREATE INDEX IF NOT EXISTS idx_student_review_logs_file_hash ON student_review_logs(file_hash);

-- Disable RLS for prototype simplicity
ALTER TABLE student_review_logs DISABLE ROW LEVEL SECURITY;


-- 3. Teacher Assignments Table
CREATE TABLE IF NOT EXISTS teacher_assignments (
    id TEXT PRIMARY KEY, -- UUID generated at publish time
    classroom_code TEXT NOT NULL,
    title TEXT NOT NULL,
    download_url TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT timezone('utc'::text, now()) NOT NULL
);

-- Indexes for search
CREATE INDEX IF NOT EXISTS idx_teacher_assignments_classroom ON teacher_assignments(classroom_code);

-- Disable RLS for prototype simplicity
ALTER TABLE teacher_assignments DISABLE ROW LEVEL SECURITY;


-- 4. User Accounts Table
CREATE TABLE IF NOT EXISTS public.user_accounts (
    username TEXT PRIMARY KEY,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL, -- 'teacher' or 'student'
    classroom_code TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT timezone('utc'::text, now()) NOT NULL
);

-- Disable RLS for schema setup; we enable it explicitly at the bottom
ALTER TABLE public.user_accounts DISABLE ROW LEVEL SECURITY;

-- 5. Active Sessions Table
CREATE TABLE IF NOT EXISTS public.active_sessions (
    session_token UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_id TEXT NOT NULL,
    role TEXT NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT timezone('utc'::text, now()) NOT NULL
);

ALTER TABLE public.active_sessions DISABLE ROW LEVEL SECURITY;


-- 6. Helper functions to read request headers (for RLS policies)
CREATE OR REPLACE FUNCTION get_current_session_token() RETURNS UUID AS $$
DECLARE
  val TEXT;
BEGIN
  val := current_setting('request.headers', true)::json->>'x-session-token';
  IF val IS NULL OR val = '' THEN
    RETURN NULL;
  END IF;
  RETURN val::uuid;
EXCEPTION
  WHEN OTHERS THEN
    RETURN NULL;
END;
$$ LANGUAGE plpgsql STABLE;


-- 7. RPC Function for User Login
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


-- 8. Unified RPC Function for Cloud Sync Endpoint
-- Receives client payload, validates student session token, upserts progress, and returns assignments.
CREATE OR REPLACE FUNCTION handle_cloud_sync(
  user_token TEXT,
  classroom_code TEXT,
  notebooks JSONB,
  logs JSONB
) RETURNS JSONB AS $$
DECLARE
  nb_record RECORD;
  log_record RECORD;
  ret_notebooks JSONB;
  v_student_username TEXT;
  v_classroom_code TEXT;
BEGIN
  -- A. Validate student session token
  SELECT entity_id, public.user_accounts.classroom_code INTO v_student_username, v_classroom_code
  FROM public.active_sessions
  JOIN public.user_accounts ON LOWER(public.user_accounts.username) = LOWER(public.active_sessions.entity_id)
  WHERE public.active_sessions.session_token = user_token::uuid
    AND public.active_sessions.role = 'student'
    AND public.active_sessions.expires_at > now();

  IF NOT FOUND THEN
    RAISE EXCEPTION 'Invalid or expired student session';
  END IF;

  -- Ensure classroom code matches session registration
  IF LOWER(v_classroom_code) <> LOWER(handle_cloud_sync.classroom_code) THEN
    RAISE EXCEPTION 'Classroom code mismatch';
  END IF;

  -- B. Upsert notebooks
  IF notebooks IS NOT NULL AND jsonb_array_length(notebooks) > 0 THEN
    FOR nb_record IN SELECT * FROM jsonb_to_recordset(notebooks) AS x(
      file_hash TEXT,
      filename TEXT,
      title TEXT,
      study_status TEXT,
      external_help_required BOOLEAN
    ) LOOP
      INSERT INTO student_notebooks (
        student_token, classroom_code, file_hash, filename, title, study_status, external_help_required, updated_at
      ) VALUES (
        v_student_username, classroom_code, nb_record.file_hash, nb_record.filename, nb_record.title, nb_record.study_status, COALESCE(nb_record.external_help_required, FALSE), now()
      ) ON CONFLICT (student_token, file_hash) DO UPDATE SET
        classroom_code = EXCLUDED.classroom_code,
        filename = EXCLUDED.filename,
        title = EXCLUDED.title,
        study_status = EXCLUDED.study_status,
        external_help_required = EXCLUDED.external_help_required,
        updated_at = now();
    END LOOP;
  END IF;

  -- C. Insert review logs
  IF logs IS NOT NULL AND jsonb_array_length(logs) > 0 THEN
    FOR log_record IN SELECT * FROM jsonb_to_recordset(logs) AS x(
      id TEXT,
      file_hash TEXT,
      page_number INTEGER,
      activity_type TEXT,
      reference_id TEXT,
      reviewed_at BIGINT,
      rating INTEGER,
      scheduled_days INTEGER,
      state_before_json TEXT,
      state_after_json TEXT
    ) LOOP
      INSERT INTO student_review_logs (
        id, student_token, classroom_code, file_hash, page_number, activity_type, reference_id, reviewed_at, rating, scheduled_days, state_before_json, state_after_json
      ) VALUES (
        log_record.id, v_student_username, classroom_code, log_record.file_hash, log_record.page_number, log_record.activity_type, log_record.reference_id, log_record.reviewed_at, log_record.rating, log_record.scheduled_days, log_record.state_before_json, log_record.state_after_json
      ) ON CONFLICT (id) DO NOTHING; -- Logs are historical logs, no updates needed
    END LOOP;
  END IF;

  -- D. Fetch teacher assignments for this classroom code
  SELECT COALESCE(jsonb_agg(jsonb_build_object(
    'id', id,
    'title', title,
    'download_url', download_url
  )), '[]'::jsonb)
  INTO ret_notebooks
  FROM teacher_assignments
  WHERE classroom_code = handle_cloud_sync.classroom_code;

  RETURN jsonb_build_object('new_notebooks', ret_notebooks);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;


-- 9. Row Level Security Policies
ALTER TABLE public.student_notebooks ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.student_review_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.teacher_assignments ENABLE ROW LEVEL SECURITY;

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

CREATE POLICY "Allow teachers to view student review logs in their classroom" ON public.student_review_logs
    FOR SELECT
    USING (
      EXISTS (
        SELECT 1 FROM public.active_sessions s
        JOIN public.user_accounts u ON LOWER(u.username) = LOWER(s.entity_id)
        WHERE s.session_token = get_current_session_token()
          AND s.expires_at > now()
          AND s.role = 'teacher'
          AND LOWER(u.classroom_code) = LOWER(student_review_logs.classroom_code)
      )
    );

CREATE POLICY "Allow teachers to view assignments in their classroom" ON public.teacher_assignments
    FOR SELECT
    USING (
      EXISTS (
        SELECT 1 FROM public.active_sessions s
        JOIN public.user_accounts u ON LOWER(u.username) = LOWER(s.entity_id)
        WHERE s.session_token = get_current_session_token()
          AND s.expires_at > now()
          AND s.role = 'teacher'
          AND LOWER(u.classroom_code) = LOWER(teacher_assignments.classroom_code)
      )
    );

CREATE POLICY "Allow teachers to insert assignments in their classroom" ON public.teacher_assignments
    FOR INSERT
    WITH CHECK (
      EXISTS (
        SELECT 1 FROM public.active_sessions s
        JOIN public.user_accounts u ON LOWER(u.username) = LOWER(s.entity_id)
        WHERE s.session_token = get_current_session_token()
          AND s.expires_at > now()
          AND s.role = 'teacher'
          AND LOWER(u.classroom_code) = LOWER(teacher_assignments.classroom_code)
      )
    );

CREATE POLICY "Allow teachers to delete assignments in their classroom" ON public.teacher_assignments
    FOR DELETE
    USING (
      EXISTS (
        SELECT 1 FROM public.active_sessions s
        JOIN public.user_accounts u ON LOWER(u.username) = LOWER(s.entity_id)
        WHERE s.session_token = get_current_session_token()
          AND s.expires_at > now()
          AND s.role = 'teacher'
          AND LOWER(u.classroom_code) = LOWER(teacher_assignments.classroom_code)
      )
    );


-- 10. Automated Purge (pg_cron)
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_cron') THEN
    PERFORM cron.schedule('nightly-cleanup', '0 0 * * *', $$ DELETE FROM public.active_sessions WHERE expires_at < NOW(); $$);
  END IF;
END $$;


-- Grant execution permissions
GRANT EXECUTE ON FUNCTION handle_cloud_sync(TEXT, TEXT, JSONB, JSONB) TO anon;
GRANT EXECUTE ON FUNCTION handle_cloud_sync(TEXT, TEXT, JSONB, JSONB) TO authenticated;
GRANT EXECUTE ON FUNCTION login_user(TEXT, TEXT, BOOLEAN) TO anon;
GRANT EXECUTE ON FUNCTION login_user(TEXT, TEXT, BOOLEAN) TO authenticated;

