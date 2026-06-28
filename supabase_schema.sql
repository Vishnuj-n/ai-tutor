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


-- 4. Unified RPC Function for Cloud Sync Endpoint
-- Receives client payload, upserts progress, writes reviews, and returns assignments.
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
BEGIN
  -- A. Upsert notebooks
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
        user_token, classroom_code, nb_record.file_hash, nb_record.filename, nb_record.title, nb_record.study_status, COALESCE(nb_record.external_help_required, FALSE), now()
      ) ON CONFLICT (student_token, file_hash) DO UPDATE SET
        classroom_code = EXCLUDED.classroom_code,
        filename = EXCLUDED.filename,
        title = EXCLUDED.title,
        study_status = EXCLUDED.study_status,
        external_help_required = EXCLUDED.external_help_required,
        updated_at = now();
    END LOOP;
  END IF;

  -- B. Insert review logs
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
        log_record.id, user_token, classroom_code, log_record.file_hash, log_record.page_number, log_record.activity_type, log_record.reference_id, log_record.reviewed_at, log_record.rating, log_record.scheduled_days, log_record.state_before_json, log_record.state_after_json
      ) ON CONFLICT (id) DO NOTHING; -- Logs are historical logs, no updates needed
    END LOOP;
  END IF;

  -- C. Fetch teacher assignments for this classroom code
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

-- Grant execution permissions
GRANT EXECUTE ON FUNCTION handle_cloud_sync(TEXT, TEXT, JSONB, JSONB) TO anon;
GRANT EXECUTE ON FUNCTION handle_cloud_sync(TEXT, TEXT, JSONB, JSONB) TO authenticated;
