package runner

import "testing"

func TestParseSettingContent(t *testing.T) {
	result := ParseSettingContent("12\n34\n56\n78")
	if result.TimeLimit != 12 {
		t.Errorf("parse time limit failed! expect: 12, actual %d", result.TimeLimit)
	}
	if result.MemoryLimit != 34 {
		t.Errorf("parse memory limit failed! expect: 34, actual %d", result.MemoryLimit)
	}
	if result.UserId != 56 {
		t.Errorf("parse user id failed! expect: 56, actual %d", result.MemoryLimit)
	}
}

func Test_parseInt(t *testing.T) {
	ret := 0
	ret = parseInt("123")
	if ret != 123 {
		t.Errorf("parse 123 failed, actual: %d", ret)
	}
}
