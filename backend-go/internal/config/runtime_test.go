package config

import "testing"

func TestRuntimeParameterBounds(t *testing.T) {
	valid := RuntimeParameters{120, 1000, 1}
	for _, p := range []RuntimeParameters{valid, {10, 1, 1}, {600, 100000, 20}} {
		if err := p.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []RuntimeParameters{{9, 1000, 1}, {601, 1000, 1}, {60, 0, 1}, {60, 100001, 1}, {60, 1000, 0}, {60, 1000, 21}} {
		if err := p.Validate(); err == nil {
			t.Fatalf("accepted invalid parameters: %+v", p)
		}
	}
	c := Config{ServerPort: "8080", LiveLogSnapshotDirectory: "unchanged"}.WithRuntime(valid)
	if c.RuntimeParameters() != valid || c.ServerPort != "8080" || c.LiveLogSnapshotDirectory != "unchanged" {
		t.Fatalf("unrelated deployment configuration changed: %+v", c)
	}
}
