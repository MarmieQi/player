package share

import (
	"testing"
	"time"

	"player-stats-backend-go/internal/apitype"
)

func TestWithFirstJoinMilestonePrependsPlayerFirstSeenTime(t *testing.T) {
	firstSeen := apitype.NewLocalDateTime(time.Date(2026, time.January, 18, 19, 42, 16, 0, time.Local))
	existing := []MilestoneItem{{Type: "FIRST_WOOD", Label: "第一次砍到木头"}}

	result := withFirstJoinMilestone(existing, &firstSeen)

	if len(result) != 2 {
		t.Fatalf("milestones length = %d, want 2", len(result))
	}
	if result[0].Type != "FIRST_JOIN" || result[0].Label != "第一次来到这里" {
		t.Fatalf("first milestone = %#v", result[0])
	}
	if result[0].FirstSeenAt != &firstSeen {
		t.Fatal("first milestone does not use the player first-seen time")
	}
	if result[0].Detail != "旅程的起点" {
		t.Fatalf("first milestone detail = %q", result[0].Detail)
	}
	if result[1].Type != "FIRST_WOOD" {
		t.Fatalf("existing milestone order changed: %#v", result)
	}
}
