# Schema

## user_profile
- id (TEXT PRIMARY KEY)
- reader_level (INT)
- reading_pace (INT)
- session_blocks (INT)
- remediation_threshold (REAL)

## notebooks
- id (TEXT PRIMARY KEY)
- name (TEXT)
- stars (INT)
- target_end_date (DATETIME)

## blocks
- id (TEXT PRIMARY KEY)
- notebook_id (TEXT)
- content (TEXT)
- start_page (INT)
- end_page (INT)
- chapter_tag (TEXT)
- word_count (INT)
- is_read (BOOLEAN)

## missions
- id (TEXT PRIMARY KEY)
- notebook_id (TEXT)
- block_id (TEXT)
- title (TEXT)
- content (TEXT)
- start_page (INT)
- end_page (INT)
- is_locked (BOOLEAN)

## cards
- id (TEXT PRIMARY KEY)
- type (TEXT)
- block_id (TEXT)
- start_page (INT)
- end_page (INT)
- front (TEXT)
- back (TEXT)
- stability (REAL)
- difficulty (REAL)
- due (DATETIME)
- state (INT)

## quiz_results
- id (TEXT PRIMARY KEY)
- mission_id (TEXT)
- score_percentage (REAL)
- passed (BOOLEAN)
- stability_hit (REAL)
- memory_collapse_triggered (BOOLEAN)
- created_at (DATETIME)

## session_states
- id (TEXT PRIMARY KEY)
- is_active (BOOLEAN)
- remaining_time_minutes (INT)
- due_cards_count (INT)
- current_phase (TEXT)
- updated_at (DATETIME)

## fsrs_updates
- id (TEXT PRIMARY KEY)
- card_id (TEXT)
- next_due (DATETIME)
- stability (REAL)
- difficulty (REAL)
- updated_at (DATETIME)
