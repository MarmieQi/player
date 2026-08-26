package bedrock

import (
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed data/names_zh_cn.json
var namesJSON []byte

var names = mustLoadNames()

func Lookup(raw string) (string, bool) {
	identifier := normalizeIdentifier(raw)
	if identifier == "" {
		return "", false
	}
	name, ok := names[identifier]
	return name, ok
}

func normalizeIdentifier(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" || value == "-" || strings.ContainsAny(value, " \t\r\n") {
		return ""
	}
	if !strings.Contains(value, ":") {
		value = "minecraft:" + value
	}
	return value
}

func mustLoadNames() map[string]string {
	result := map[string]string{}
	if err := json.Unmarshal(namesJSON, &result); err != nil {
		panic("invalid embedded Bedrock Chinese name catalog: " + err.Error())
	}
	if len(result) == 0 {
		panic("embedded Bedrock Chinese name catalog is empty")
	}
	return result
}
