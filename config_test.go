package job

import (
	"testing"
	"time"
)

func TestConfigNameIsJob(t *testing.T) {
	if got := (Config{}).ConfigName(); got != "job" {
		t.Fatalf("ConfigName() = %q, want job", got)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		ok   bool
	}{
		{name: "zero means the default", cfg: Config{}, ok: true},
		{name: "a positive timeout", cfg: Config{Timeout: time.Second}, ok: true},
		{name: "a negative timeout", cfg: Config{Timeout: -time.Second}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()

			if tc.ok && err != nil {
				t.Errorf("rejected a valid config: %v", err)
			}
			if !tc.ok && err == nil {
				t.Error("accepted an invalid config")
			}
		})
	}
}

func TestSetDefaults(t *testing.T) {
	var cfg Config
	cfg.SetDefaults()

	if cfg.Timeout != defaultTimeout {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, defaultTimeout)
	}
}
