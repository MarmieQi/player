package main

import "testing"

func TestIsOfficialSource(t *testing.T) {
	for _, origin := range []string{
		"https://github.com/Mojang/bedrock-samples.git",
		"git@github.com:Mojang/bedrock-samples.git",
	} {
		if !isOfficialSource(origin) {
			t.Errorf("isOfficialSource(%q) = false", origin)
		}
	}
	if isOfficialSource("https://github.com/example/bedrock-samples.git") {
		t.Fatal("forked repository should not be accepted as the official source")
	}
}
