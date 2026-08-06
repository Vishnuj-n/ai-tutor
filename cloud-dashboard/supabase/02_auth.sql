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
  
  -- Unlink classroom code from student user account
  UPDATE public.user_accounts SET classroom_code = '' WHERE LOWER(username) = LOWER(p_student_username) AND role = 'student';

  RETURN jsonb_build_object('success', true, 'removed_student', LOWER(p_student_username));
END;
$$ LANGUAGE plpgsql SECURITY DEFINER SET search_path = public;

GRANT EXECUTE ON FUNCTION login_user(TEXT, TEXT, BOOLEAN) TO anon, authenticated;
GRANT EXECUTE ON FUNCTION signup_user(TEXT, TEXT, TEXT, TEXT) TO anon, authenticated;
GRANT EXECUTE ON FUNCTION toggle_classroom_lock(TEXT, BOOLEAN) TO authenticated;
GRANT EXECUTE ON FUNCTION remove_student_from_classroom(TEXT, TEXT) TO authenticated;
