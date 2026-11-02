package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"kaapi/api/internal/platform/config"
)

func TestLoadDotEnvNeverOverridesTheRealEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	content := "# comment\nKAAPI_T_A=from-file\nexport KAAPI_T_B=\"quoted value\"\nKAAPI_T_C=from-file\nnot a setting\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KAAPI_T_C", "from-env")
	os.Unsetenv("KAAPI_T_A")
	os.Unsetenv("KAAPI_T_B")
	t.Cleanup(func() { os.Unsetenv("KAAPI_T_A"); os.Unsetenv("KAAPI_T_B") })

	if err := config.LoadDotEnv(path); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("KAAPI_T_A") != "from-file" || os.Getenv("KAAPI_T_B") != "quoted value" || os.Getenv("KAAPI_T_C") != "from-env" {
		t.Fatalf("A=%q B=%q C=%q", os.Getenv("KAAPI_T_A"), os.Getenv("KAAPI_T_B"), os.Getenv("KAAPI_T_C"))
	}
	if err := config.LoadDotEnv(filepath.Join(t.TempDir(), "missing")); err != nil {
		t.Fatal("a missing file is fine")
	}
}
