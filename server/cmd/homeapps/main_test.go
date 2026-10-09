package main

import "testing"

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestFlagsOverrideEnvOverrideDefaults(t *testing.T) {
	c, err := parseFlags(nil, env(nil))
	if err != nil || c.port != 8686 || c.host != "0.0.0.0" || c.hostmanURL != "http://127.0.0.1:8899" {
		t.Errorf("defaults = %+v, %v", c, err)
	}
	e := env(map[string]string{"PORT": "9000", "HOMEAPPS_HOSTMAN": "http://pi5:8899"})
	if c, _ = parseFlags(nil, e); c.port != 9000 || c.hostmanURL != "http://pi5:8899" {
		t.Errorf("env not used: %+v", c)
	}
	if c, _ = parseFlags([]string{"--port", "9100", "--hostman", "http://x:1"}, e); c.port != 9100 || c.hostmanURL != "http://x:1" {
		t.Errorf("flags should win over env: %+v", c)
	}
}

func TestFlagErrors(t *testing.T) {
	if _, err := parseFlags(nil, env(map[string]string{"PORT": "abc"})); err == nil {
		t.Error("bad PORT accepted")
	}
	if _, err := parseFlags([]string{"--port", "70000"}, env(nil)); err == nil {
		t.Error("out-of-range port accepted")
	}
}
