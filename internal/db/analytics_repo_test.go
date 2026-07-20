package db

import (
	"os"
	"testing"
)

func TestAnalyticsLifecycle(t *testing.T) {
	tempDB := "test_analytics_lifecycle.db"
	_ = os.Remove(tempDB)
	defer func() { _ = os.Remove(tempDB) }()

	repo, err := Init(tempDB, "")
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer func() {
		_ = repo.Close()
	}()

	// 1. Telemetry should do nothing when AnalyticsEnabled is false
	err = repo.TrackAnalyticsEvent("reading_complete", "hash-123", 4, `{"duration": 10}`)
	if err != nil {
		t.Fatalf("TrackAnalyticsEvent failed on disabled state: %v", err)
	}

	payloads, _, err := repo.GetUnsyncedAnalyticsEvents()
	if err != nil {
		t.Fatalf("GetUnsyncedAnalyticsEvents failed: %v", err)
	}
	if len(payloads) != 0 {
		t.Errorf("expected 0 analytics events, got %d", len(payloads))
	}

	// 2. Enable telemetry tracking
	settings, err := repo.GetUserSettings()
	if err != nil {
		t.Fatalf("GetUserSettings failed: %v", err)
	}
	settings.AnalyticsEnabled = true
	if err := repo.UpdateUserSettings(*settings); err != nil {
		t.Fatalf("UpdateUserSettings failed: %v", err)
	}

	// 3. Track events
	err = repo.TrackAnalyticsEvent("reading_complete", "hash-123", 4, `{"duration": 10}`)
	if err != nil {
		t.Fatalf("TrackAnalyticsEvent failed: %v", err)
	}
	err = repo.TrackAnalyticsEvent("quiz_complete", "hash-123", 0, `{"score": 80}`)
	if err != nil {
		t.Fatalf("TrackAnalyticsEvent failed: %v", err)
	}

	// 4. Fetch unsynced
	payloads, ids, err := repo.GetUnsyncedAnalyticsEvents()
	if err != nil {
		t.Fatalf("GetUnsyncedAnalyticsEvents failed: %v", err)
	}
	if len(payloads) != 2 {
		t.Fatalf("expected 2 analytics events, got %d", len(payloads))
	}

	if payloads[0].EventType != "reading_complete" || payloads[0].FileHash != "hash-123" || payloads[0].PageNumber != 4 {
		t.Errorf("unexpected first event payload: %+v", payloads[0])
	}
	if payloads[1].EventType != "quiz_complete" || payloads[1].Metadata != `{"score": 80}` {
		t.Errorf("unexpected second event payload: %+v", payloads[1])
	}

	// 5. Mark synced
	if err := repo.MarkAnalyticsSynced(ids); err != nil {
		t.Fatalf("MarkAnalyticsSynced failed: %v", err)
	}

	// 6. Fetch unsynced again (should be 0)
	payloads, _, err = repo.GetUnsyncedAnalyticsEvents()
	if err != nil {
		t.Fatalf("GetUnsyncedAnalyticsEvents failed: %v", err)
	}
	if len(payloads) != 0 {
		t.Errorf("expected 0 analytics events after mark synced, got %d", len(payloads))
	}
}
