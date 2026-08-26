package importer

import "testing"

func TestLogQueryRowAddsDisplayNamesWithoutChangingRawDetails(t *testing.T) {
	values := []string{
		"2026-08-26", "12:00:00", "Steve", "放置方块",
		"1", "2", "3", "overworld", "4", "5", "6", "overworld",
		"minecraft:cobblestone", "plain text",
	}
	row := logQueryRow(RemoteLogFile{FileName: "player_actions_2026-08-26.csv"}, 7, values)
	if row.Detail1 != "minecraft:cobblestone" || row.Detail1Display != "圆石" {
		t.Fatalf("detail1 = %q, display = %q", row.Detail1, row.Detail1Display)
	}
	if row.Detail2 != "plain text" || row.Detail2Display != "" {
		t.Fatalf("detail2 = %q, display = %q", row.Detail2, row.Detail2Display)
	}
}

func TestLogQueryRowExposesLegacyBucketClickCoordinatesAndBucketName(t *testing.T) {
	values := []string{
		"2026-08-25", "00:02:12", "Alex", "使用桶放置",
		"-34264", "21", "40104", "主世界", "-", "-", "-", "-",
		"minecraft:acacia_planks", "minecraft:water_bucket",
	}
	row := logQueryRow(RemoteLogFile{FileName: "player_actions_2026-08-25.csv"}, 484, values)
	if row.X2 != "-34264" || row.Y2 != "21" || row.Z2 != "40104" || row.Dimension2 != "主世界" {
		t.Fatalf("interaction coordinates = %q, %q, %q, %q", row.X2, row.Y2, row.Z2, row.Dimension2)
	}
	if row.Detail1Display != "金合欢木板" || row.Detail2Display != "水桶" {
		t.Fatalf("display names = %q, %q", row.Detail1Display, row.Detail2Display)
	}
}

func TestLogQueryRowKeepsExplicitBucketInteractionCoordinates(t *testing.T) {
	values := []string{
		"2026-08-25", "12:00:00", "Alex", "使用桶交互",
		"999", "999", "999", "主世界", "10", "20", "30", "主世界",
		"minecraft:stone", "minecraft:water_bucket",
	}
	row := logQueryRow(RemoteLogFile{}, 1, values)
	if row.X2 != "10" || row.Y2 != "20" || row.Z2 != "30" {
		t.Fatalf("explicit interaction coordinates were overwritten: %#v", row)
	}
}
