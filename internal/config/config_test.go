package config

import (
	"os"
	"testing"
)

// unsetEnv 清掉所有 GOODHABIT_* 环境变量，保证默认值测试不受宿主环境影响。
func unsetEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"GOODHABIT_ADDR", "GOODHABIT_DBPATH", "GOODHABIT_DAY_TZ",
		"GOODHABIT_TRUST_PROXY", "GOODHABIT_SECURE_COOKIE", "GOODHABIT_READ_POOL",
	} {
		os.Unsetenv(k)
	}
}

func TestLoadDefaults(t *testing.T) {
	unsetEnv(t)
	c, err := Load(nil)
	if err != nil {
		t.Fatalf("Load(nil) error: %v", err)
	}
	if c.Addr != "127.0.0.1:8080" {
		t.Errorf("Addr = %q, want 127.0.0.1:8080", c.Addr)
	}
	if c.DBPath != "good-habit.db" {
		t.Errorf("DBPath = %q, want good-habit.db", c.DBPath)
	}
	if c.ReadPool != 5 {
		t.Errorf("ReadPool = %d, want 5", c.ReadPool)
	}
	if c.TrustProxy || c.SecureCookie {
		t.Errorf("TrustProxy/SecureCookie 默认应为 false")
	}
}

func TestLoadEnvOverride(t *testing.T) {
	t.Setenv("GOODHABIT_DBPATH", "x.db")
	t.Setenv("GOODHABIT_READ_POOL", "6")
	c, err := Load(nil)
	if err != nil {
		t.Fatalf("Load(nil) error: %v", err)
	}
	if c.DBPath != "x.db" {
		t.Errorf("DBPath = %q, want x.db", c.DBPath)
	}
	if c.ReadPool != 6 {
		t.Errorf("ReadPool = %d, want 6", c.ReadPool)
	}
}

func TestReadPoolClamp(t *testing.T) {
	t.Setenv("GOODHABIT_READ_POOL", "2")
	c, err := Load(nil)
	if err != nil {
		t.Fatalf("Load(nil) error: %v", err)
	}
	if c.ReadPool != 4 {
		t.Errorf("ReadPool = %d, want clamp 到 4", c.ReadPool)
	}
	t.Setenv("GOODHABIT_READ_POOL", "99")
	c, err = Load(nil)
	if err != nil {
		t.Fatalf("Load(nil) error: %v", err)
	}
	if c.ReadPool != 6 {
		t.Errorf("ReadPool = %d, want clamp 到 6", c.ReadPool)
	}
}

func TestFlagOverridesEnv(t *testing.T) {
	t.Setenv("GOODHABIT_DBPATH", "x.db")
	c, err := Load([]string{"-db", "y.db", "-addr", "127.0.0.1:9999"})
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if c.DBPath != "y.db" {
		t.Errorf("DBPath = %q, want y.db（flag 覆盖 env）", c.DBPath)
	}
	if c.Addr != "127.0.0.1:9999" {
		t.Errorf("Addr = %q, want 127.0.0.1:9999", c.Addr)
	}
}
