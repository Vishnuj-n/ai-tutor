-- ==============================================================================
-- MASTER SUPABASE SETUP SCRIPT FOR AI TUTOR CLOUD DASHBOARD
-- Copy and paste this ENTIRE file into Supabase SQL Editor and click 'RUN'.
-- ==============================================================================

-- 1. Extensions & Tables
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA extensions;

CREATE TABLE IF NOT EXISTS public.student_notebooks (
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
CREATE INDEX IF NOT EXISTS idx_student_notebooks_classroom ON public.student_notebooks(classroom_code);
CREATE INDEX IF NOT EXISTS idx_student_notebooks_student ON public.student_notebooks(student_token);

CREATE TABLE IF NOT EXISTS public.student_review_logs (
    id TEXT PRIMARY KEY,
    student_token TEXT NOT NULL,
    classroom_code TEXT NOT NULL,
    file_hash TEXT NOT NULL,
    page_number INTEGER NOT NULL,
    activity_type TEXT NOT NULL,
    reference_id TEXT NOT NULL,
    reviewed_at BIGINT NOT NULL,
    rating INTEGER NOT NULL,
    scheduled_days INTEGER NOT NULL,
    state_before_json TEXT,
    state_after_json TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT timezone('utc'::text, now()) NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_student_review_logs_classroom ON public.student_review_logs(classroom_code);
CREATE INDEX IF NOT EXISTS idx_student_review_logs_student ON public.student_review_logs(student_token);
CREATE INDEX IF NOT EXISTS idx_student_review_logs_file_hash ON public.student_review_logs(file_hash);

CREATE TABLE IF NOT EXISTS public.teacher_assignments (
    id TEXT PRIMARY KEY,
    classroom_code TEXT NOT NULL,
    title TEXT NOT NULL,
    download_url TEXT NOT NULL CHECK (download_url ILIKE 'http://%' OR download_url ILIKE 'https://%'),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT timezone('utc'::text, now()) NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_teacher_assignments_classroom ON public.teacher_assignments(classroom_code);

CREATE TABLE IF NOT EXISTS public.user_accounts (
    username TEXT PRIMARY KEY,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL,
    classroom_code TEXT NOT NULL,
    is_locked BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT timezone('utc'::text, now()) NOT NULL
);

CREATE TABLE IF NOT EXISTS public.active_sessions (
    session_token UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_id TEXT NOT NULL,
    role TEXT NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT timezone('utc'::text, now()) NOT NULL
);

CREATE TABLE IF NOT EXISTS public.teacher_signup_invites (
    classroom_code TEXT NOT NULL,
    invite_email_or_username TEXT NOT NULL,
    is_used BOOLEAN DEFAULT FALSE NOT NULL,
    PRIMARY KEY (classroom_code, invite_email_or_username)
);

CREATE TABLE IF NOT EXISTS public.anonymous_analytics_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type TEXT NOT NULL CONSTRAINT check_event_type CHECK (event_type IN ('reading_complete', 'quiz_complete')),
    file_hash TEXT DEFAULT '',
    page_number INTEGER DEFAULT 0,
    metadata JSONB CONSTRAINT check_metadata_size CHECK (octet_length(metadata::text) <= 2048),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT timezone('utc'::text, now()) NOT NULL
);
CREATE INDEX IF NOT EXISTS anonymous_analytics_events_created_at_idx ON public.anonymous_analytics_events (created_at);

-- 2. Essential Helper & Auth Functions
CREATE OR REPLACE FUNCTION get_current_session_token() RETURNS UUID AS $$
DECLARE
  val TEXT;
BEGIN
  val := current_setting('request.headers', true)::json->>'x-session-token';
  IF val IS NULL OR val = '' THEN RETURN NULL; END IF;
  RETURN val::uuid;
EXCEPTION WHEN OTHERS THEN RETURN NULL;
END;
$$ LANGUAGE plpgsql STABLE SET search_path = public;

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
    AND password_hash = extensions.crypt(p_password, password_hash);

  IF NOT FOUND THEN RAISE EXCEPTION 'Invalid username or password'; END IF;

  IF p_is_desktop THEN
    v_expires := now() + interval '10 years';
  ELSE
    v_expires := now() + interval '24 hours';
  END IF;

  v_token := gen_random_uuid();
  INSERT INTO public.active_sessions (session_token, entity_id, role, expires_at)
  VALUES (v_token, LOWER(p_username), v_role, v_expires);

  RETURN jsonb_build_object('session_token', v_token, 'role', v_role, 'classroom_code', v_class_code, 'username', LOWER(p_username));
END;
$$ LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, extensions;

CREATE OR REPLACE FUNCTION signup_user(
  p_username TEXT,
  p_password TEXT,
  p_role TEXT,
  p_classroom_code TEXT
) RETURNS JSONB AS $$
BEGIN
  IF p_username IS NULL OR p_username = '' THEN RAISE EXCEPTION 'Username is required'; END IF;
  IF p_password IS NULL OR length(p_password) < 6 THEN RAISE EXCEPTION 'Password must be at least 6 characters'; END IF;
  IF p_role IS NULL OR (p_role <> 'student' AND p_role <> 'teacher') THEN RAISE EXCEPTION 'Invalid role specified'; END IF;
  IF EXISTS (SELECT 1 FROM public.user_accounts WHERE LOWER(username) = LOWER(p_username)) THEN RAISE EXCEPTION 'Username is already registered'; END IF;

  IF p_role = 'teacher' THEN
    IF EXISTS (
      SELECT 1 FROM public.teacher_signup_invites
      WHERE classroom_code = UPPER(p_classroom_code) AND LOWER(invite_email_or_username) = LOWER(p_username) AND is_used = FALSE
    ) THEN
      UPDATE public.teacher_signup_invites SET is_used = TRUE
      WHERE classroom_code = UPPER(p_classroom_code) AND LOWER(invite_email_or_username) = LOWER(p_username);
    END IF;
  ELSIF p_role = 'student' THEN
    IF NOT EXISTS (SELECT 1 FROM public.user_accounts WHERE classroom_code = UPPER(p_classroom_code) AND role = 'teacher') THEN
      RAISE EXCEPTION 'Classroom code must belong to an existing registered teacher';
    END IF;

    IF EXISTS (
      SELECT 1 FROM public.user_accounts
      WHERE classroom_code = UPPER(p_classroom_code) AND role = 'teacher' AND is_locked = TRUE
    ) THEN
      RAISE EXCEPTION 'Classroom is currently locked by the teacher. New student joins are disabled.';
    END IF;
  END IF;

  INSERT INTO public.user_accounts (username, password_hash, role, classroom_code)
  VALUES (LOWER(p_username), extensions.crypt(p_password, extensions.gen_salt('bf')), p_role, UPPER(p_classroom_code));

  RETURN jsonb_build_object('success', true, 'username', LOWER(p_username));
END;
$$ LANGUAGE plpgsql SECURITY DEFINER SET search_path = public, extensions;

CREATE OR REPLACE FUNCTION toggle_classroom_lock(
  p_classroom_code TEXT,
  p_is_locked BOOLEAN
) RETURNS JSONB AS $$
DECLARE
  v_teacher_username TEXT;
  v_teacher_class_code TEXT;
  v_session_token UUID;
BEGIN
  v_session_token := get_current_session_token();
  IF v_session_token IS NULL THEN RAISE EXCEPTION 'Missing session token'; END IF;

  SELECT entity_id, public.user_accounts.classroom_code INTO v_teacher_username, v_teacher_class_code
  FROM public.active_sessions
  JOIN public.user_accounts ON LOWER(public.user_accounts.username) = LOWER(public.active_sessions.entity_id)
  WHERE public.active_sessions.session_token = v_session_token
    AND public.active_sessions.role = 'teacher'
    AND public.active_sessions.expires_at > now();

  IF NOT FOUND THEN RAISE EXCEPTION 'Invalid or expired teacher session'; END IF;
  IF LOWER(v_teacher_class_code) <> LOWER(p_classroom_code) THEN RAISE EXCEPTION 'Classroom code mismatch'; END IF;

  UPDATE public.user_accounts
  SET is_locked = p_is_locked
  WHERE LOWER(username) = LOWER(v_teacher_username) AND role = 'teacher';

  RETURN jsonb_build_object('success', true, 'is_locked', p_is_locked);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER SET search_path = public;

CREATE OR REPLACE FUNCTION remove_student_from_classroom(
  p_student_username TEXT,
  p_classroom_code TEXT
) RETURNS JSONB AS $$
DECLARE
  v_teacher_username TEXT;
  v_teacher_class_code TEXT;
  v_session_token UUID;
BEGIN
  v_session_token := get_current_session_token();
  IF v_session_token IS NULL THEN RAISE EXCEPTION 'Missing session token'; END IF;

  SELECT entity_id, public.user_accounts.classroom_code INTO v_teacher_username, v_teacher_class_code
  FROM public.active_sessions
  JOIN public.user_accounts ON LOWER(public.user_accounts.username) = LOWER(public.active_sessions.entity_id)
  WHERE public.active_sessions.session_token = v_session_token
    AND public.active_sessions.role = 'teacher'
    AND public.active_sessions.expires_at > now();

  IF NOT FOUND THEN RAISE EXCEPTION 'Invalid or expired teacher session'; END IF;
  IF LOWER(v_teacher_class_code) <> LOWER(p_classroom_code) THEN RAISE EXCEPTION 'Classroom code mismatch'; END IF;

  DELETE FROM public.student_notebooks WHERE LOWER(student_token) = LOWER(p_student_username) AND classroom_code = p_classroom_code;
  DELETE FROM public.student_review_logs WHERE LOWER(student_token) = LOWER(p_student_username) AND classroom_code = p_classroom_code;
  
  UPDATE public.user_accounts SET classroom_code = '' WHERE LOWER(username) = LOWER(p_student_username) AND role = 'student';

  RETURN jsonb_build_object('success', true, 'removed_student', LOWER(p_student_username));
END;
$$ LANGUAGE plpgsql SECURITY DEFINER SET search_path = public;

-- 3. Data Sync & Dashboard RPC Functions
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
  SELECT entity_id, public.user_accounts.classroom_code INTO v_student_username, v_classroom_code
  FROM public.active_sessions
  JOIN public.user_accounts ON LOWER(public.user_accounts.username) = LOWER(public.active_sessions.entity_id)
  WHERE public.active_sessions.session_token = user_token::uuid
    AND public.active_sessions.role = 'student'
    AND public.active_sessions.expires_at > now();

  IF NOT FOUND THEN RAISE EXCEPTION 'Invalid or expired student session'; END IF;
  IF LOWER(v_classroom_code) <> LOWER(handle_cloud_sync.classroom_code) THEN RAISE EXCEPTION 'Classroom code mismatch'; END IF;

  IF notebooks IS NOT NULL AND jsonb_array_length(notebooks) > 0 THEN
    FOR nb_record IN SELECT * FROM jsonb_to_recordset(notebooks) AS x(
      file_hash TEXT, filename TEXT, title TEXT, study_status TEXT, external_help_required BOOLEAN
    ) LOOP
      INSERT INTO student_notebooks (
        student_token, classroom_code, file_hash, filename, title, study_status, external_help_required, updated_at
      ) VALUES (
        v_student_username, classroom_code, nb_record.file_hash, nb_record.filename, nb_record.title, nb_record.study_status, COALESCE(nb_record.external_help_required, FALSE), now()
      ) ON CONFLICT (student_token, file_hash) DO UPDATE SET
        classroom_code = EXCLUDED.classroom_code, filename = EXCLUDED.filename, title = EXCLUDED.title, study_status = EXCLUDED.study_status, external_help_required = EXCLUDED.external_help_required, updated_at = now();
    END LOOP;
  END IF;

  IF logs IS NOT NULL AND jsonb_array_length(logs) > 0 THEN
    FOR log_record IN SELECT * FROM jsonb_to_recordset(logs) AS x(
      id TEXT, file_hash TEXT, page_number INTEGER, activity_type TEXT, reference_id TEXT, reviewed_at BIGINT, rating INTEGER, scheduled_days INTEGER, state_before_json TEXT, state_after_json TEXT
    ) LOOP
      INSERT INTO student_review_logs (
        id, student_token, classroom_code, file_hash, page_number, activity_type, reference_id, reviewed_at, rating, scheduled_days, state_before_json, state_after_json
      ) VALUES (
        log_record.id, v_student_username, classroom_code, log_record.file_hash, log_record.page_number, log_record.activity_type, log_record.reference_id, log_record.reviewed_at, log_record.rating, log_record.scheduled_days, log_record.state_before_json, log_record.state_after_json
      ) ON CONFLICT (id) DO NOTHING;
    END LOOP;
  END IF;

  SELECT COALESCE(jsonb_agg(jsonb_build_object(
    'id', id, 'title', title, 'download_url', download_url
  )), '[]'::jsonb) INTO ret_notebooks
  FROM teacher_assignments WHERE classroom_code = handle_cloud_sync.classroom_code;

  RETURN jsonb_build_object('new_notebooks', ret_notebooks);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER SET search_path = public;

CREATE OR REPLACE FUNCTION get_classroom_dashboard(p_classroom_code text)
RETURNS json LANGUAGE plpgsql SECURITY DEFINER SET search_path = public AS $$
declare
    v_result json;
    v_teacher_username TEXT;
    v_teacher_class_code TEXT;
    v_session_token UUID;
begin
    v_session_token := get_current_session_token();
    if v_session_token is null then raise exception 'Missing session token'; end if;

    select entity_id, public.user_accounts.classroom_code into v_teacher_username, v_teacher_class_code
    from public.active_sessions
    join public.user_accounts on lower(public.user_accounts.username) = lower(public.active_sessions.entity_id)
    where public.active_sessions.session_token = v_session_token
      and public.active_sessions.role = 'teacher'
      and public.active_sessions.expires_at > now();

    if not found then raise exception 'Invalid or expired teacher session'; end if;
    if lower(v_teacher_class_code) <> lower(p_classroom_code) then raise exception 'Classroom code mismatch'; end if;

    with student_nbs as (
        select student_token, json_agg(n order by updated_at desc) as notebooks, count(*) filter (where external_help_required = true) as alerts_count, max(extract(epoch from updated_at) * 1000) as max_nb_update
        from student_notebooks n where classroom_code = p_classroom_code group by student_token
    ),
    student_logs as (
        select student_token, json_agg(l order by reviewed_at desc) as logs, max(reviewed_at * 1000) as max_log_update
        from student_review_logs l where classroom_code = p_classroom_code group by student_token
    ),
    all_students as (
        select distinct student_token from (
            select student_token from student_notebooks where classroom_code = p_classroom_code
            union
            select student_token from student_review_logs where classroom_code = p_classroom_code
        ) s
    ),
    rolled_up as (
        select a.student_token as token, coalesce(n.notebooks, '[]'::json) as notebooks, coalesce(l.logs, '[]'::json) as logs, coalesce(n.alerts_count, 0) as "alertsCount", greatest(coalesce(n.max_nb_update, 0), coalesce(l.max_log_update, 0)) as "lastUpdate"
        from all_students a
        left join student_nbs n on n.student_token = a.student_token
        left join student_logs l on l.student_token = a.student_token
        order by "lastUpdate" desc
    )
    select json_build_object(
        'is_locked', coalesce((select is_locked from public.user_accounts where lower(username) = lower(v_teacher_username) and role = 'teacher'), false),
        'students', coalesce(json_agg(r), '[]'::json)
    ) into v_result from rolled_up r;
    return v_result;
end;
$$;

-- 4. Enable Row Level Security (RLS)
ALTER TABLE public.student_notebooks ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.student_review_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.teacher_assignments ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.user_accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.active_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.teacher_signup_invites ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.anonymous_analytics_events ENABLE ROW LEVEL SECURITY;

-- 5. RLS Policies
DROP POLICY IF EXISTS "Allow teachers to view student notebooks in their classroom" ON public.student_notebooks;
CREATE POLICY "Allow teachers to view student notebooks in their classroom" ON public.student_notebooks
    FOR SELECT USING (
      EXISTS (
        SELECT 1 FROM public.active_sessions s JOIN public.user_accounts u ON LOWER(u.username) = LOWER(s.entity_id)
        WHERE s.session_token = get_current_session_token() AND s.expires_at > now() AND s.role = 'teacher' AND LOWER(u.classroom_code) = LOWER(student_notebooks.classroom_code)
      )
    );

DROP POLICY IF EXISTS "Allow teachers to view student review logs in their classroom" ON public.student_review_logs;
CREATE POLICY "Allow teachers to view student review logs in their classroom" ON public.student_review_logs
    FOR SELECT USING (
      EXISTS (
        SELECT 1 FROM public.active_sessions s JOIN public.user_accounts u ON LOWER(u.username) = LOWER(s.entity_id)
        WHERE s.session_token = get_current_session_token() AND s.expires_at > now() AND s.role = 'teacher' AND LOWER(u.classroom_code) = LOWER(student_review_logs.classroom_code)
      )
    );

DROP POLICY IF EXISTS "Allow teachers to view assignments in their classroom" ON public.teacher_assignments;
CREATE POLICY "Allow teachers to view assignments in their classroom" ON public.teacher_assignments
    FOR SELECT USING (
      EXISTS (
        SELECT 1 FROM public.active_sessions s JOIN public.user_accounts u ON LOWER(u.username) = LOWER(s.entity_id)
        WHERE s.session_token = get_current_session_token() AND s.expires_at > now() AND s.role = 'teacher' AND LOWER(u.classroom_code) = LOWER(teacher_assignments.classroom_code)
      )
    );

DROP POLICY IF EXISTS "Allow teachers to insert assignments in their classroom" ON public.teacher_assignments;
CREATE POLICY "Allow teachers to insert assignments in their classroom" ON public.teacher_assignments
    FOR INSERT WITH CHECK (
      EXISTS (
        SELECT 1 FROM public.active_sessions s JOIN public.user_accounts u ON LOWER(u.username) = LOWER(s.entity_id)
        WHERE s.session_token = get_current_session_token() AND s.expires_at > now() AND s.role = 'teacher' AND LOWER(u.classroom_code) = LOWER(teacher_assignments.classroom_code)
      )
    );

DROP POLICY IF EXISTS "Allow teachers to delete assignments in their classroom" ON public.teacher_assignments;
CREATE POLICY "Allow teachers to delete assignments in their classroom" ON public.teacher_assignments
    FOR DELETE USING (
      EXISTS (
        SELECT 1 FROM public.active_sessions s JOIN public.user_accounts u ON LOWER(u.username) = LOWER(s.entity_id)
        WHERE s.session_token = get_current_session_token() AND s.expires_at > now() AND s.role = 'teacher' AND LOWER(u.classroom_code) = LOWER(teacher_assignments.classroom_code)
      )
    );

DROP POLICY IF EXISTS "No direct client access to active_sessions" ON public.active_sessions;
CREATE POLICY "No direct client access to active_sessions" ON public.active_sessions FOR ALL USING (false);
DROP POLICY IF EXISTS "No direct client access to user_accounts" ON public.user_accounts;
CREATE POLICY "No direct client access to user_accounts" ON public.user_accounts FOR ALL USING (false);
DROP POLICY IF EXISTS "No direct client access to teacher_signup_invites" ON public.teacher_signup_invites;
CREATE POLICY "No direct client access to teacher_signup_invites" ON public.teacher_signup_invites FOR ALL USING (false);

DROP POLICY IF EXISTS "Allow anonymous inserts" ON public.anonymous_analytics_events;
CREATE POLICY "Allow anonymous inserts" ON public.anonymous_analytics_events FOR INSERT TO anon WITH CHECK (true);

-- 6. Grants (Crucial for Anon & Authenticated Roles)
GRANT USAGE ON SCHEMA public TO anon, authenticated;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO anon, authenticated;

GRANT EXECUTE ON FUNCTION login_user(TEXT, TEXT, BOOLEAN) TO anon, authenticated;
GRANT EXECUTE ON FUNCTION signup_user(TEXT, TEXT, TEXT, TEXT) TO anon, authenticated;
GRANT EXECUTE ON FUNCTION toggle_classroom_lock(TEXT, BOOLEAN) TO authenticated;
GRANT EXECUTE ON FUNCTION remove_student_from_classroom(TEXT, TEXT) TO authenticated;
GRANT EXECUTE ON FUNCTION get_classroom_dashboard(TEXT) TO anon, authenticated;
GRANT EXECUTE ON FUNCTION handle_cloud_sync(TEXT, TEXT, JSONB, JSONB) TO anon, authenticated;
GRANT EXECUTE ON FUNCTION get_current_session_token() TO anon, authenticated;

-- 7. Storage Bucket & Policies
INSERT INTO storage.buckets (id, name, public, file_size_limit, allowed_mime_types)
VALUES ('assignments', 'assignments', true, 52428800, ARRAY['application/pdf'])
ON CONFLICT (id) DO UPDATE SET file_size_limit = EXCLUDED.file_size_limit, allowed_mime_types = EXCLUDED.allowed_mime_types;

DROP POLICY IF EXISTS "Teacher PDF Upload Policy" ON storage.objects;
CREATE POLICY "Teacher PDF Upload Policy" ON storage.objects FOR INSERT TO public
WITH CHECK (bucket_id = 'assignments' AND (storage.extension(name) = 'pdf') AND EXISTS (
  SELECT 1 FROM public.active_sessions s WHERE s.session_token = get_current_session_token() AND s.role = 'teacher' AND s.expires_at > now()
));

DROP POLICY IF EXISTS "Public Read Assignments Policy" ON storage.objects;
CREATE POLICY "Public Read Assignments Policy" ON storage.objects FOR SELECT TO public USING (bucket_id = 'assignments');
