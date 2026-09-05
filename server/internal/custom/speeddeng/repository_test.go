package speeddeng

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCustomRecordsUseDedicatedTableNames(t *testing.T) {
	if (sessionRecord{}).TableName() != "custom_speed_deng_sessions" {
		t.Fatalf("session table = %q, want custom_speed_deng_sessions", (sessionRecord{}).TableName())
	}
	if (eventRecord{}).TableName() != "custom_speed_deng_events" {
		t.Fatalf("event table = %q, want custom_speed_deng_events", (eventRecord{}).TableName())
	}
}

func TestEventRecordSnapshotsSpeedDengFields(t *testing.T) {
	cost := 0.125
	input := CaptureInput{
		SessionID:          uuid.New(),
		SourceRequestLogID: uuid.New(),
		SourceRequestID:    "req-123",
		SiteID:             uuid.New(),
		SiteName:           "Codex",
		ModelKey:           "gpt-5",
		APIKeyID:           uuid.New(),
		APIKeyName:         " Wilson ",
		PromptTokens:       10,
		CompletionTokens:   5,
		CachedTokens:       2,
		TotalTokens:        15,
		EstimatedCostUSD:   &cost,
		Currency:           "USD",
		CreatedAt:          time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC),
	}
	record := eventRecordFromInput(input)
	if record.SessionID != input.SessionID || record.SourceRequestLogID != input.SourceRequestLogID || record.APIKeyName != " Wilson " {
		t.Fatalf("record identity/name = %#v, want input snapshot", record)
	}
	if record.EstimatedCostUSD == nil || *record.EstimatedCostUSD != cost || record.TotalTokens != 15 {
		t.Fatalf("record usage = %#v, want cost/tokens snapshot", record)
	}
}

func TestSpeedDengDisplayNameNormalizesUnknownAndAvoidsDuplicateSuffix(t *testing.T) {
	tests := []struct {
		input string
		key   string
		label string
	}{
		{input: "Wilson（本体）", key: "wilson::speed_deng", label: "Wilson-雷速蹬"},
		{input: "Wilson-雷速蹬", key: "wilson::speed_deng", label: "Wilson-雷速蹬"},
		{input: " none ", key: "__unknown__::speed_deng", label: "未识别-雷速蹬"},
	}
	for _, tc := range tests {
		key, label := speedDengDisplayName(tc.input)
		if key != tc.key || label != tc.label {
			t.Fatalf("speedDengDisplayName(%q) = %q/%q, want %q/%q", tc.input, key, label, tc.key, tc.label)
		}
	}
}
