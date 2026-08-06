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

GRANT EXECUTE ON FUNCTION get_classroom_dashboard(TEXT) TO authenticated;
GRANT EXECUTE ON FUNCTION handle_cloud_sync(TEXT, TEXT, JSONB, JSONB) TO anon, authenticated;
