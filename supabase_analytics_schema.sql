-- Supabase Schema for AI Tutor Anonymous Analytics
-- Create the telemetry table and configure Row Level Security policies.

CREATE TABLE IF NOT EXISTS public.anonymous_analytics_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type TEXT NOT NULL CONSTRAINT check_event_type CHECK (event_type IN ('page_view', 'reading_complete', 'quiz_complete', 'flashcard_review')),
    file_hash TEXT DEFAULT '',
    page_number INTEGER DEFAULT 0,
    metadata JSONB CONSTRAINT check_metadata_size CHECK (octet_length(metadata::text) < 2048),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT timezone('utc'::text, now()) NOT NULL
);

-- Enable RLS on the telemetry table
ALTER TABLE public.anonymous_analytics_events ENABLE ROW LEVEL SECURITY;

-- 1. Allow public/anonymous users to insert records
CREATE POLICY "Allow anonymous inserts" 
ON public.anonymous_analytics_events 
FOR INSERT 
TO anon 
WITH CHECK (true);

-- 2. Explicitly deny read / update / delete to anonymous users
-- Anonymous clients can only append data, they can never select or modify it.
CREATE POLICY "Deny anonymous reads" 
ON public.anonymous_analytics_events 
FOR SELECT 
TO anon 
USING (false);

CREATE POLICY "Deny anonymous updates" 
ON public.anonymous_analytics_events 
FOR UPDATE 
TO anon 
USING (false)
WITH CHECK (false);

CREATE POLICY "Deny anonymous deletes" 
ON public.anonymous_analytics_events 
FOR DELETE 
TO anon 
USING (false);

-- 3. Setup Automated Purge (90-day retention) via pg_cron
DO $outer$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_cron') THEN
    PERFORM cron.schedule(
      'purge-old-analytics-events',
      '0 1 * * *', -- Everyday at 1:00 AM UTC
      $job$
      DELETE FROM public.anonymous_analytics_events
      WHERE created_at < NOW() - INTERVAL '90 days';
      $job$
    );
  END IF;
END;
$outer$;
