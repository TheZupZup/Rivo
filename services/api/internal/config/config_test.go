package config

import "testing"

func TestLoadUsesDefaultsWhenTheEnvironmentIsEmpty(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.HTTPAddress != ":8080" {
		t.Fatalf("unexpected default address %q", cfg.HTTPAddress)
	}
	if cfg.MaxUploadBytes != defaultMaxUploadBytes {
		t.Fatalf("unexpected default upload limit %d", cfg.MaxUploadBytes)
	}
	if cfg.UploadRateLimit.Burst <= 0 || cfg.UploadRateLimit.RefillPerMinute <= 0 {
		t.Fatalf("expected a usable default rate limit, got %+v", cfg.UploadRateLimit)
	}
}

func TestLoadReadsTheUploadLimit(t *testing.T) {
	t.Setenv("MAX_UPLOAD_BYTES", "2048")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.MaxUploadBytes != 2048 {
		t.Fatalf("expected 2048, got %d", cfg.MaxUploadBytes)
	}
}

// A typo in an operational limit must stop the process, not silently fall back to
// a default that is several orders of magnitude more permissive.
func TestLoadRefusesInvalidLimits(t *testing.T) {
	cases := map[string]map[string]string{
		"non numeric upload limit": {"MAX_UPLOAD_BYTES": "1GiB"},
		"zero upload limit":        {"MAX_UPLOAD_BYTES": "0"},
		"negative upload limit":    {"MAX_UPLOAD_BYTES": "-1"},
		"zero burst":               {"UPLOAD_RATE_LIMIT_BURST": "0"},
		"negative refill":          {"UPLOAD_RATE_LIMIT_PER_MINUTE": "-5"},
	}

	for name, environment := range cases {
		t.Run(name, func(t *testing.T) {
			for key, value := range environment {
				t.Setenv(key, value)
			}

			if _, err := Load(); err == nil {
				t.Fatal("expected an error rather than a silent fallback")
			}
		})
	}
}
