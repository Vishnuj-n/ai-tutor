ALTER TABLE public.student_notebooks ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.student_review_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.teacher_assignments ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.user_accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.active_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.teacher_signup_invites ENABLE ROW LEVEL SECURITY;

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
