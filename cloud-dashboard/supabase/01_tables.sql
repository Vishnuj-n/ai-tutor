CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA extensions;

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
CREATE INDEX IF NOT EXISTS idx_student_notebooks_classroom ON student_notebooks(classroom_code);
CREATE INDEX IF NOT EXISTS idx_student_notebooks_student ON student_notebooks(student_token);

CREATE TABLE IF NOT EXISTS student_review_logs (
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
CREATE INDEX IF NOT EXISTS idx_student_review_logs_classroom ON student_review_logs(classroom_code);
CREATE INDEX IF NOT EXISTS idx_student_review_logs_student ON student_review_logs(student_token);
CREATE INDEX IF NOT EXISTS idx_student_review_logs_file_hash ON student_review_logs(file_hash);

CREATE TABLE IF NOT EXISTS teacher_assignments (
    id TEXT PRIMARY KEY,
    classroom_code TEXT NOT NULL,
    title TEXT NOT NULL,
    download_url TEXT NOT NULL CHECK (download_url ILIKE 'http://%' OR download_url ILIKE 'https://%'),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT timezone('utc'::text, now()) NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_teacher_assignments_classroom ON teacher_assignments(classroom_code);

CREATE TABLE IF NOT EXISTS public.user_accounts (
    username TEXT PRIMARY KEY,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL,
    classroom_code TEXT NOT NULL,
    is_locked BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT timezone('utc'::text, now()) NOT NULL
);
ALTER TABLE public.user_accounts ADD COLUMN IF NOT EXISTS is_locked BOOLEAN NOT NULL DEFAULT FALSE;

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
