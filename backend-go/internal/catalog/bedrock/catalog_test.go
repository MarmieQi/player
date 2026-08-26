package bedrock

import (
	_ "embed"
	"encoding/json"
	"strings"
	"testing"
)

//go:embed data/source.json
var sourceJSON []byte

type catalogSourceInfo struct {
	CatalogVersion    int    `json:"catalogVersion"`
	SourceTag         string `json:"sourceTag"`
	SourceCommit      string `json:"sourceCommit"`
	UniqueIdentifiers int    `json:"uniqueIdentifiers"`
	MappedIdentifiers int    `json:"mappedIdentifiers"`
}

func TestLookupUsesGeneratedBedrockChineseNames(t *testing.T) {
	tests := map[string]string{
		"minecraft:cobblestone":            "圆石",
		"cobblestone":                      "圆石",
		"minecraft:acacia_double_slab":     "金合欢木双层台阶",
		"minecraft:water_bucket":           "水桶",
		"minecraft:axolotl_bucket":         "美西螈桶",
		"minecraft:music_disc_pigstep":     "音乐唱片（莉娜·雷恩 - Pigstep）",
		"minecraft:hard_red_stained_glass": "红色强化玻璃",
		"minecraft:underwater_tnt":         "水下 TNT",
	}
	for identifier, want := range tests {
		got, ok := Lookup(identifier)
		if !ok || got != want {
			t.Errorf("Lookup(%q) = %q, %v, want %q, true", identifier, got, ok, want)
		}
	}
}

func TestLookupRejectsUnknownOrFreeText(t *testing.T) {
	for _, value := range []string{"", "-", "some player", "custom:unknown_block"} {
		if got, ok := Lookup(value); ok || got != "" {
			t.Errorf("Lookup(%q) = %q, %v, want empty, false", value, got, ok)
		}
	}
}

func TestGeneratedCatalogMetadataAndEntriesAreComplete(t *testing.T) {
	var info catalogSourceInfo
	if err := json.Unmarshal(sourceJSON, &info); err != nil {
		t.Fatalf("decode embedded source metadata: %v", err)
	}
	if info.CatalogVersion != 1 || info.SourceTag == "" || info.SourceCommit == "" {
		t.Fatalf("invalid source metadata: %#v", info)
	}
	if info.UniqueIdentifiers != info.MappedIdentifiers || info.MappedIdentifiers != len(names) {
		t.Fatalf("catalog coverage mismatch: metadata=%#v count=%d", info, len(names))
	}
	for identifier, name := range names {
		if !strings.HasPrefix(identifier, "minecraft:") {
			t.Errorf("non-Minecraft identifier in generated catalog: %q", identifier)
		}
		if strings.TrimSpace(name) == "" {
			t.Errorf("empty generated name for %q", identifier)
		}
	}
}
