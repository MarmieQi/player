package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateResolvesSerializationAndEnglishTokenNames(t *testing.T) {
	source, overrides := writeFixture(t,
		`{"data_items":[{"name":"minecraft:cobblestone","serialization_id":"tile.cobblestone"}]}`,
		`{"data_items":[{"name":"minecraft:water_bucket","serialization_id":"item.bucketWater"},{"name":"minecraft:axolotl_bucket","serialization_id":"item.unmatched"}]}`,
		"tile.cobblestone.name=Cobblestone\nitem.bucketWater.name=Water Bucket\nitem.bucketAxolotl.name=Bucket of Axolotl\n",
		"tile.cobblestone.name=圆石\nitem.bucketWater.name=水桶\nitem.bucketAxolotl.name=美西螈桶\n",
		`{}`,
	)
	result, err := Generate(Options{SourceDirectory: source, OverridesPath: overrides})
	if err != nil {
		t.Fatal(err)
	}
	for identifier, want := range map[string]string{
		"minecraft:cobblestone":    "圆石",
		"minecraft:water_bucket":   "水桶",
		"minecraft:axolotl_bucket": "美西螈桶",
	} {
		if got := result.Names[identifier]; got != want {
			t.Errorf("generated %s = %q, want %q", identifier, got, want)
		}
	}
	if result.UniqueIdentifiers != 3 || len(result.Names) != 3 {
		t.Fatalf("unexpected result counts: %#v", result)
	}
}

func TestGenerateFailsWhenAnyVanillaIdentifierIsUnmapped(t *testing.T) {
	source, overrides := writeFixture(t,
		`{"data_items":[{"name":"minecraft:not_translated","serialization_id":"tile.not_translated"}]}`,
		`{"data_items":[{"name":"minecraft:water_bucket","serialization_id":"item.bucketWater"}]}`,
		"tile.other.name=Other\nitem.bucketWater.name=Water Bucket\n",
		"tile.other.name=其他\nitem.bucketWater.name=水桶\n",
		`{}`,
	)
	_, err := Generate(Options{SourceDirectory: source, OverridesPath: overrides})
	if err == nil || !strings.Contains(err.Error(), "minecraft:not_translated") {
		t.Fatalf("Generate() error = %v, want missing identifier error", err)
	}
}

func TestGenerateRejectsDuplicateVanillaIdentifiers(t *testing.T) {
	source, overrides := writeFixture(t,
		`{"data_items":[{"name":"minecraft:cobblestone","serialization_id":"tile.cobblestone"},{"name":"minecraft:cobblestone","serialization_id":"tile.cobblestone"}]}`,
		`{"data_items":[{"name":"minecraft:water_bucket","serialization_id":"item.bucketWater"}]}`,
		"tile.cobblestone.name=Cobblestone\nitem.bucketWater.name=Water Bucket\n",
		"tile.cobblestone.name=圆石\nitem.bucketWater.name=水桶\n",
		`{}`,
	)
	_, err := Generate(Options{SourceDirectory: source, OverridesPath: overrides})
	if err == nil || !strings.Contains(err.Error(), "duplicate identifier minecraft:cobblestone") {
		t.Fatalf("Generate() error = %v, want duplicate identifier error", err)
	}
}

func TestGenerateRejectsIncompleteVanillaEntries(t *testing.T) {
	tests := []struct {
		name      string
		blocks    string
		wantError string
	}{
		{
			name:      "empty identifier",
			blocks:    `{"data_items":[{"name":"","serialization_id":"tile.cobblestone"}]}`,
			wantError: "empty identifier",
		},
		{
			name:      "invalid namespace",
			blocks:    `{"data_items":[{"name":"cobblestone","serialization_id":"tile.cobblestone"}]}`,
			wantError: `invalid identifier "cobblestone"`,
		},
		{
			name:      "empty serialization ID",
			blocks:    `{"data_items":[{"name":"minecraft:cobblestone","serialization_id":""}]}`,
			wantError: "empty serialization ID",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source, overrides := writeFixture(t,
				test.blocks,
				`{"data_items":[{"name":"minecraft:water_bucket","serialization_id":"item.bucketWater"}]}`,
				"tile.cobblestone.name=Cobblestone\nitem.bucketWater.name=Water Bucket\n",
				"tile.cobblestone.name=圆石\nitem.bucketWater.name=水桶\n",
				`{}`,
			)
			_, err := Generate(Options{SourceDirectory: source, OverridesPath: overrides})
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("Generate() error = %v, want error containing %q", err, test.wantError)
			}
		})
	}
}

func writeFixture(t *testing.T, blocks, items, english, chinese, overrides string) (string, string) {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		blocksPath:  blocks,
		itemsPath:   items,
		englishPath: english,
		chinesePath: chinese,
	}
	for relative, content := range files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	overridesPath := filepath.Join(root, "overrides.json")
	if err := os.WriteFile(overridesPath, []byte(overrides), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, overridesPath
}
