package runner

import (
	"encoding/json"
	"testing"
)

func TestParseSettingContent(t *testing.T) {
	origin := Setting{
		TimeLimit: 12,
		MemoryLimit: 34,
		Language: 2,
	}
	content, _ := json.Marshal(origin)
	result := ParseSettingContent(content)
	if result.TimeLimit != 12 {
		t.Errorf("parse time limit failed! expect: 12, actual %d", result.TimeLimit)
	}
	if result.MemoryLimit != 34 {
		t.Errorf("parse memory limit failed! expect: 34, actual %d", result.MemoryLimit)
	}
	if result.Language != 2 {
		t.Errorf("parse language id failed! expect: 56, actual %d", result.Language)
	}
}
