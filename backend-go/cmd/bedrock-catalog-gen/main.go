package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"player-stats-backend-go/internal/catalog/bedrock/generator"
)

const sourceRepository = "https://github.com/Mojang/bedrock-samples.git"

func main() {
	source := flag.String("source", "", "path to a pinned Mojang/bedrock-samples checkout")
	overrides := flag.String("overrides", "internal/catalog/bedrock/data/overrides_zh_cn.json", "manual override file")
	namesOutput := flag.String("names-output", "internal/catalog/bedrock/data/names_zh_cn.json", "generated name catalog")
	sourceOutput := flag.String("source-output", "internal/catalog/bedrock/data/source.json", "generated source metadata")
	flag.Parse()

	if strings.TrimSpace(*source) == "" {
		fatalf("-source is required")
	}
	sourceDirectory, err := filepath.Abs(*source)
	if err != nil {
		fatalf("resolve source path: %v", err)
	}
	origin := gitOutput(sourceDirectory, "remote", "get-url", "origin")
	if !isOfficialSource(origin) {
		fatalf("source repository must be %s, got %s", sourceRepository, origin)
	}
	if status := gitOutput(sourceDirectory, "status", "--porcelain"); status != "" {
		fatalf("source repository has local changes; use a clean, pinned checkout")
	}
	tag := gitOutput(sourceDirectory, "describe", "--tags", "--exact-match")
	commit := gitOutput(sourceDirectory, "rev-parse", "HEAD")
	version := strings.TrimPrefix(tag, "v")

	options := generator.Options{
		SourceDirectory:  sourceDirectory,
		OverridesPath:    *overrides,
		MinecraftVersion: version,
		SourceTag:        tag,
		SourceCommit:     commit,
		SourceRepository: sourceRepository,
	}
	result, err := generator.Generate(options)
	if err != nil {
		fatalf("generate catalog: %v", err)
	}
	if err := generator.WriteResult(result, options, *namesOutput, *sourceOutput); err != nil {
		fatalf("write catalog: %v", err)
	}
	fmt.Printf("generated %d Chinese names from %d block and %d item entries\n", len(result.Names), result.BlockIdentifiers, result.ItemIdentifiers)
}

func isOfficialSource(origin string) bool {
	value := strings.ToLower(strings.TrimSpace(origin))
	value = strings.TrimSuffix(value, ".git")
	value = strings.TrimPrefix(value, "git@github.com:")
	value = strings.TrimPrefix(value, "https://github.com/")
	value = strings.TrimPrefix(value, "http://github.com/")
	return value == "mojang/bedrock-samples"
}

func gitOutput(directory string, args ...string) string {
	command := exec.Command("git", append([]string{"-C", directory}, args...)...)
	output, err := command.Output()
	if err != nil {
		fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(output))
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
