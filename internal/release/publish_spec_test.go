package release

import "testing"

func TestEmbeddedPublishConfig(t *testing.T) {
	cfg, err := loadPublishConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UploadRetryAttempts < 1 || cfg.UploadRetryDelay <= 0 {
		t.Fatalf("invalid publish retry policy: %+v", cfg)
	}
	if !cfg.ReleaseNotFound.Match([]byte("Error: could not find release app v1.2.3")) ||
		cfg.ReleaseNotFound.Match([]byte("upload failed: Error: could not find release app v1.2.3")) {
		t.Fatalf("release-not-found pattern is not anchored: %q", cfg.ReleaseNotFound)
	}
}
