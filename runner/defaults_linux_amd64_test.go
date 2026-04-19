//go:build linux && amd64

package runner

import "testing"

func TestLoadConfigAddsAMD64DefaultCalls(t *testing.T) {
	restoreGlobals := preserveConfigTestGlobals()
	defer restoreGlobals()

	runWithTempCaseJSON(t, `{}`, func() {
		SetLogger(nil)

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}

		for _, name := range []string{"arch_prctl", "readlink", "access", "readlinkat", "faccessat"} {
			if !containsString(cfg.AllowedCalls, name) {
				t.Fatalf("AllowedCalls missing %q: %v", name, cfg.AllowedCalls)
			}
		}
	})
}
