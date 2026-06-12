package db

import (
	"ai-tutor/internal/models"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestProfileAndSettingsLifecycle(t *testing.T) {
	tempDB := "test_profile_lifecycle.db"
	_ = os.Remove(tempDB)
	defer os.Remove(tempDB)

	if err := Init(tempDB, ""); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer Close()

	// 1. Test GetUserSettings default state
	s, err := GetUserSettings()
	if err != nil {
		t.Fatalf("GetUserSettings failed: %v", err)
	}
	if s.DailyStudyMinutes != 90 {
		t.Errorf("expected default 90 minutes, got %d", s.DailyStudyMinutes)
	}
	if s.ActiveProfileID != "" {
		t.Errorf("expected empty active profile, got %q", s.ActiveProfileID)
	}

	// 2. Test CreateProfile
	deadline := time.Now().AddDate(0, 3, 0).Unix() // 3 months from now
	p1 := models.StudyProfile{
		ID:         "prof-1",
		Name:       "UPSC prep",
		DeadlineAt: deadline,
	}
	if err := CreateProfile(p1); err != nil {
		t.Fatalf("CreateProfile failed: %v", err)
	}

	// 3. Test GetProfiles
	profiles, err := GetProfiles()
	if err != nil {
		t.Fatalf("GetProfiles failed: %v", err)
	}
	if len(profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(profiles))
	}
	if profiles[0].Name != "UPSC prep" {
		t.Errorf("expected UPSC prep, got %q", profiles[0].Name)
	}

	// 4. Test UpdateUserSettings with active profile
	s.ActiveProfileID = "prof-1"
	s.DailyStudyMinutes = 120
	s.SkipToReadingActive = true
	s.CloudSyncURL = "http://localhost/sync"
	s.CloudAPIToken = "secret-token"

	if err := UpdateUserSettings(*s); err != nil {
		t.Fatalf("UpdateUserSettings failed: %v", err)
	}

	sUpdated, err := GetUserSettings()
	if err != nil {
		t.Fatalf("GetUserSettings failed: %v", err)
	}
	if sUpdated.DailyStudyMinutes != 120 {
		t.Errorf("expected 120 minutes, got %d", sUpdated.DailyStudyMinutes)
	}
	if sUpdated.ActiveProfileID != "prof-1" {
		t.Errorf("expected active profile prof-1, got %q", sUpdated.ActiveProfileID)
	}
	if !sUpdated.SkipToReadingActive {
		t.Errorf("expected skip_to_reading_active to be true")
	}
	if sUpdated.CloudSyncURL != "http://localhost/sync" {
		t.Errorf("expected cloud_sync_url to be http://localhost/sync, got %q", sUpdated.CloudSyncURL)
	}

	// 5. Test Notebook Shelf Gating (limit of 4 active textbooks)
	// Create a notebook and assign to profile
	err = CreateNotebook("nb-1", "Polity Book", "path/1.pdf", "pdf", "", 10)
	if err != nil {
		t.Fatalf("failed to create notebook: %v", err)
	}

	err = AssignNotebookToProfile("nb-1", "prof-1")
	if err != nil {
		t.Fatalf("failed to assign notebook to profile: %v", err)
	}

	// Activate notebook 1
	err = UpdateNotebookStudyStatus("nb-1", "active")
	if err != nil {
		t.Fatalf("failed to activate nb-1: %v", err)
	}

	// Create 4 more notebooks and try to activate them all (bringing total active to 5)
	for i := 2; i <= 5; i++ {
		id := fmt.Sprintf("nb-%d", i)
		err = CreateNotebook(id, fmt.Sprintf("Book %d", i), "path.pdf", "pdf", "", 10)
		if err != nil {
			t.Fatalf("failed to create notebook %s: %v", id, err)
		}
		err = AssignNotebookToProfile(id, "prof-1")
		if err != nil {
			t.Fatalf("failed to assign notebook %s: %v", id, err)
		}
	}

	// Activate notebooks 2, 3, 4 (total active = 4)
	for i := 2; i <= 4; i++ {
		id := fmt.Sprintf("nb-%d", i)
		err = UpdateNotebookStudyStatus(id, "active")
		if err != nil {
			t.Fatalf("failed to activate notebook %s: %v", id, err)
		}
	}

	// Notebook 5 should fail activation since limit of 4 active is hit
	err = UpdateNotebookStudyStatus("nb-5", "active")
	if err == nil {
		t.Errorf("expected activation to fail because profile already has 4 active notebooks")
	}

	// 6. Test DeleteProfile cleans up references
	if err := DeleteProfile("prof-1"); err != nil {
		t.Fatalf("DeleteProfile failed: %v", err)
	}

	// Notebooks should now have profile_id = NULL
	nb1, err := GetNotebookByID("nb-1")
	if err != nil {
		t.Fatalf("GetNotebookByID failed: %v", err)
	}
	if nb1.ProfileID != "" {
		t.Errorf("expected profile_id to be empty after profile deletion, got %q", nb1.ProfileID)
	}
}
