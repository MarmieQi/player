package generator

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	blocksPath  = "metadata/vanilladata_modules/mojang-blocks.json"
	itemsPath   = "metadata/vanilladata_modules/mojang-items.json"
	englishPath = "resource_pack/texts/en_US.lang"
	chinesePath = "resource_pack/texts/zh_CN.lang"
)

type Options struct {
	SourceDirectory  string
	OverridesPath    string
	MinecraftVersion string
	SourceTag        string
	SourceCommit     string
	SourceRepository string
}

type Result struct {
	Names             map[string]string
	BlockIdentifiers  int
	ItemIdentifiers   int
	UniqueIdentifiers int
}

type SourceInfo struct {
	CatalogVersion    int      `json:"catalogVersion"`
	MinecraftVersion  string   `json:"minecraftVersion"`
	SourceTag         string   `json:"sourceTag"`
	SourceCommit      string   `json:"sourceCommit"`
	SourceRepository  string   `json:"sourceRepository"`
	SourceFiles       []string `json:"sourceFiles"`
	BlockIdentifiers  int      `json:"blockIdentifiers"`
	ItemIdentifiers   int      `json:"itemIdentifiers"`
	UniqueIdentifiers int      `json:"uniqueIdentifiers"`
	MappedIdentifiers int      `json:"mappedIdentifiers"`
}

type vanillaModule struct {
	DataItems []vanillaEntry `json:"data_items"`
}

type vanillaEntry struct {
	Name            string `json:"name"`
	SerializationID string `json:"serialization_id"`
}

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

func Generate(options Options) (Result, error) {
	if strings.TrimSpace(options.SourceDirectory) == "" {
		return Result{}, errors.New("source directory is required")
	}
	blocks, err := loadModule(filepath.Join(options.SourceDirectory, filepath.FromSlash(blocksPath)))
	if err != nil {
		return Result{}, err
	}
	items, err := loadModule(filepath.Join(options.SourceDirectory, filepath.FromSlash(itemsPath)))
	if err != nil {
		return Result{}, err
	}
	english, err := loadLang(filepath.Join(options.SourceDirectory, filepath.FromSlash(englishPath)))
	if err != nil {
		return Result{}, err
	}
	chinese, err := loadLang(filepath.Join(options.SourceDirectory, filepath.FromSlash(chinesePath)))
	if err != nil {
		return Result{}, err
	}
	overrides, err := loadOverrides(options.OverridesPath)
	if err != nil {
		return Result{}, err
	}

	englishIndex := indexEnglishNames(english, chinese)
	allIdentifiers := map[string]struct{}{}
	names := map[string]string{}

	// Item entries are loaded first. Block labels then take precedence for IDs that
	// can represent both an inventory item and a placed block in PlayerLogger rows.
	for _, entry := range items.DataItems {
		allIdentifiers[entry.Name] = struct{}{}
		if name, ok := resolveEntry(entry, englishIndex, chinese); ok {
			names[entry.Name] = name
		}
	}
	for _, entry := range blocks.DataItems {
		allIdentifiers[entry.Name] = struct{}{}
		if name, ok := resolveEntry(entry, englishIndex, chinese); ok {
			names[entry.Name] = name
		}
	}

	for identifier := range allIdentifiers {
		if _, ok := names[identifier]; ok {
			continue
		}
		if name, ok := deriveKnownName(identifier, chinese); ok {
			names[identifier] = name
		}
	}
	for identifier, name := range overrides {
		if _, ok := allIdentifiers[identifier]; !ok {
			return Result{}, fmt.Errorf("override identifier is not present in vanilla data: %s", identifier)
		}
		if strings.TrimSpace(name) == "" {
			return Result{}, fmt.Errorf("override name is empty: %s", identifier)
		}
		names[identifier] = strings.TrimSpace(name)
	}

	missing := make([]string, 0)
	for identifier := range allIdentifiers {
		if strings.TrimSpace(names[identifier]) == "" {
			missing = append(missing, identifier)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		return Result{}, fmt.Errorf("%d vanilla identifiers are missing Chinese names:\n%s", len(missing), strings.Join(missing, "\n"))
	}

	return Result{
		Names:             names,
		BlockIdentifiers:  len(blocks.DataItems),
		ItemIdentifiers:   len(items.DataItems),
		UniqueIdentifiers: len(allIdentifiers),
	}, nil
}

func WriteResult(result Result, options Options, namesPath, sourcePath string) error {
	if err := writeJSON(namesPath, result.Names); err != nil {
		return err
	}
	info := SourceInfo{
		CatalogVersion:    1,
		MinecraftVersion:  options.MinecraftVersion,
		SourceTag:         options.SourceTag,
		SourceCommit:      options.SourceCommit,
		SourceRepository:  options.SourceRepository,
		SourceFiles:       []string{blocksPath, itemsPath, englishPath, chinesePath},
		BlockIdentifiers:  result.BlockIdentifiers,
		ItemIdentifiers:   result.ItemIdentifiers,
		UniqueIdentifiers: result.UniqueIdentifiers,
		MappedIdentifiers: len(result.Names),
	}
	return writeJSON(sourcePath, info)
}

func loadModule(path string) (vanillaModule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return vanillaModule{}, fmt.Errorf("read vanilla data %s: %w", path, err)
	}
	var module vanillaModule
	if err := json.Unmarshal(data, &module); err != nil {
		return vanillaModule{}, fmt.Errorf("decode vanilla data %s: %w", path, err)
	}
	if len(module.DataItems) == 0 {
		return vanillaModule{}, fmt.Errorf("vanilla data has no entries: %s", path)
	}
	seen := make(map[string]struct{}, len(module.DataItems))
	for index, entry := range module.DataItems {
		identifier := strings.TrimSpace(entry.Name)
		if identifier == "" {
			return vanillaModule{}, fmt.Errorf("vanilla data entry %d has an empty identifier: %s", index, path)
		}
		if identifier != entry.Name || !strings.HasPrefix(identifier, "minecraft:") {
			return vanillaModule{}, fmt.Errorf("vanilla data entry %d has an invalid identifier %q: %s", index, entry.Name, path)
		}
		if strings.TrimSpace(entry.SerializationID) == "" {
			return vanillaModule{}, fmt.Errorf("vanilla data entry %s has an empty serialization ID: %s", identifier, path)
		}
		if _, ok := seen[identifier]; ok {
			return vanillaModule{}, fmt.Errorf("vanilla data contains duplicate identifier %s: %s", identifier, path)
		}
		seen[identifier] = struct{}{}
	}
	return module, nil
}

func loadLang(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open language file %s: %w", path, err)
	}
	defer file.Close()

	values := map[string]string{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if line == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		separator := strings.IndexByte(line, '=')
		if separator <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:separator])
		value := line[separator+1:]
		if comment := strings.Index(value, "\t#"); comment >= 0 {
			value = value[:comment]
		}
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			values[key] = value
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan language file %s: %w", path, err)
	}
	return values, nil
}

func loadOverrides(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read overrides %s: %w", path, err)
	}
	result := map[string]string{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode overrides %s: %w", path, err)
	}
	return result, nil
}

func indexEnglishNames(english, chinese map[string]string) map[string][]string {
	result := map[string][]string{}
	for key, value := range english {
		if _, ok := chinese[key]; !ok || (!strings.HasPrefix(key, "tile.") && !strings.HasPrefix(key, "item.")) || !strings.HasSuffix(key, ".name") {
			continue
		}
		canonical := canonicalTokens(value)
		if canonical != "" {
			result[canonical] = append(result[canonical], key)
		}
	}
	return result
}

func resolveEntry(entry vanillaEntry, englishIndex map[string][]string, chinese map[string]string) (string, bool) {
	identifier := strings.TrimSpace(entry.Name)
	short := strings.TrimPrefix(identifier, "minecraft:")
	serializationID := strings.TrimSpace(entry.SerializationID)
	candidates := []string{serializationID}
	if serializationID != "" && !strings.HasSuffix(serializationID, ".name") {
		candidates = append(candidates, serializationID+".name")
	}
	candidates = append(candidates, "tile."+short+".name", "item."+short+".name")
	if strings.HasSuffix(short, "_spawn_egg") {
		entity := strings.TrimSuffix(short, "_spawn_egg")
		candidates = append(candidates, "item.spawn_egg.entity."+entity+".name")
	}
	for _, key := range candidates {
		if name := strings.TrimSpace(chinese[key]); name != "" {
			return name, true
		}
	}

	canonical := canonicalTokens(strings.ReplaceAll(short, "_", " "))
	keys := englishIndex[canonical]
	if len(keys) == 1 {
		return strings.TrimSpace(chinese[keys[0]]), true
	}
	return "", false
}

func canonicalTokens(value string) string {
	value = strings.ToLower(value)
	value = nonAlphanumeric.ReplaceAllString(value, " ")
	parts := strings.Fields(value)
	filtered := parts[:0]
	for _, part := range parts {
		switch part {
		case "of", "the", "with":
			continue
		default:
			filtered = append(filtered, part)
		}
	}
	sort.Strings(filtered)
	return strings.Join(filtered, " ")
}

func deriveKnownName(identifier string, chinese map[string]string) (string, bool) {
	short := strings.TrimPrefix(identifier, "minecraft:")
	if strings.HasPrefix(short, "element_") {
		atomicNumber, err := strconv.Atoi(strings.TrimPrefix(short, "element_"))
		if err == nil && atomicNumber >= 0 && atomicNumber < len(elementNames) {
			return elementNames[atomicNumber], true
		}
	}
	if name, ok := deriveColoredName(short); ok {
		return name, true
	}
	if name, ok := derivePoplarName(short); ok {
		return name, true
	}
	if strings.HasPrefix(short, "music_disc_") {
		serializationID := "item.record_" + strings.TrimPrefix(short, "music_disc_")
		base := strings.TrimSpace(chinese["item.record.name"])
		detail := strings.TrimSpace(chinese[serializationID+".desc"])
		if base != "" && detail != "" {
			return base + "（" + detail + "）", true
		}
	}
	return "", false
}

type colorName struct {
	ID    string
	Label string
}

var colorNames = []colorName{
	{ID: "light_blue", Label: "淡蓝色"},
	{ID: "light_gray", Label: "淡灰色"},
	{ID: "magenta", Label: "品红色"},
	{ID: "orange", Label: "橙色"},
	{ID: "yellow", Label: "黄色"},
	{ID: "purple", Label: "紫色"},
	{ID: "white", Label: "白色"},
	{ID: "black", Label: "黑色"},
	{ID: "brown", Label: "棕色"},
	{ID: "green", Label: "绿色"},
	{ID: "blue", Label: "蓝色"},
	{ID: "cyan", Label: "青色"},
	{ID: "gray", Label: "灰色"},
	{ID: "lime", Label: "黄绿色"},
	{ID: "pink", Label: "粉红色"},
	{ID: "red", Label: "红色"},
}

func deriveColoredName(short string) (string, bool) {
	if strings.HasPrefix(short, "colored_torch_") {
		colorID := strings.TrimPrefix(short, "colored_torch_")
		if label, ok := colorLabel(colorID); ok {
			return label + "火把", true
		}
	}
	if strings.HasPrefix(short, "hard_") {
		rest := strings.TrimPrefix(short, "hard_")
		if rest == "glass" {
			return "强化玻璃", true
		}
		if rest == "glass_pane" {
			return "强化玻璃板", true
		}
		for _, color := range colorNames {
			prefix := color.ID + "_stained_"
			if rest == prefix+"glass" {
				return color.Label + "强化玻璃", true
			}
			if rest == prefix+"glass_pane" {
				return color.Label + "强化玻璃板", true
			}
		}
	}
	for _, color := range colorNames {
		prefix := color.ID + "_"
		if !strings.HasPrefix(short, prefix) {
			continue
		}
		switch strings.TrimPrefix(short, prefix) {
		case "cushion":
			return color.Label + "坐垫", true
		case "wool_slab":
			return color.Label + "羊毛台阶", true
		case "wool_double_slab":
			return color.Label + "羊毛双层台阶", true
		case "wool_stairs":
			return color.Label + "羊毛楼梯", true
		}
	}
	return "", false
}

func colorLabel(id string) (string, bool) {
	for _, color := range colorNames {
		if color.ID == id {
			return color.Label, true
		}
	}
	return "", false
}

func derivePoplarName(short string) (string, bool) {
	labels := map[string]string{
		"poplar_boat":           "杨木船",
		"poplar_button":         "杨木按钮",
		"poplar_chest_boat":     "杨木运输船",
		"poplar_door":           "杨木门",
		"poplar_double_slab":    "杨木双层台阶",
		"poplar_fence":          "杨木栅栏",
		"poplar_fence_gate":     "杨木栅栏门",
		"poplar_hanging_sign":   "悬挂式杨木告示牌",
		"poplar_log":            "杨木原木",
		"poplar_planks":         "杨木木板",
		"poplar_pressure_plate": "杨木压力板",
		"poplar_sapling":        "杨树树苗",
		"poplar_shelf":          "杨木架",
		"poplar_sign":           "杨木告示牌",
		"poplar_slab":           "杨木台阶",
		"poplar_stairs":         "杨木楼梯",
		"poplar_standing_sign":  "杨木告示牌",
		"poplar_trapdoor":       "杨木活板门",
		"poplar_wall_sign":      "墙上的杨木告示牌",
		"poplar_wood":           "杨木",
		"stripped_poplar_log":   "去皮杨木原木",
		"stripped_poplar_wood":  "去皮杨木",
		"orange_poplar_leaves":  "橙色杨树树叶",
		"red_poplar_leaves":     "红色杨树树叶",
		"yellow_poplar_leaves":  "黄色杨树树叶",
	}
	name, ok := labels[short]
	return name, ok
}

var elementNames = []string{
	"未知元素", "氢", "氦", "锂", "铍", "硼", "碳", "氮", "氧", "氟", "氖",
	"钠", "镁", "铝", "硅", "磷", "硫", "氯", "氩", "钾", "钙",
	"钪", "钛", "钒", "铬", "锰", "铁", "钴", "镍", "铜", "锌",
	"镓", "锗", "砷", "硒", "溴", "氪", "铷", "锶", "钇", "锆",
	"铌", "钼", "锝", "钌", "铑", "钯", "银", "镉", "铟", "锡",
	"锑", "碲", "碘", "氙", "铯", "钡", "镧", "铈", "镨", "钕",
	"钷", "钐", "铕", "钆", "铽", "镝", "钬", "铒", "铥", "镱",
	"镥", "铪", "钽", "钨", "铼", "锇", "铱", "铂", "金", "汞",
	"铊", "铅", "铋", "钋", "砹", "氡", "钫", "镭", "锕", "钍",
	"镤", "铀", "镎", "钚", "镅", "锔", "锫", "锎", "锿", "镄",
	"钔", "锘", "铹", "𬬻", "𬭊", "𬭳", "𬭛", "𬭶", "鿏", "𫟼",
	"𬬭", "鿔", "鿭", "𫓧", "镆", "𫟷", "鿬", "鿫",
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create output directory for %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
