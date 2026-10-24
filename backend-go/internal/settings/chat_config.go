package settings

import (
	"path"
	"path/filepath"
	"player-stats-backend-go/internal/auth"
	"strings"
)

type ChatDirectory struct {
	Enabled      bool   `json:"enabled"`
	SMBDirectory string `json:"smbDirectory"`
	FileGlob     string `json:"fileGlob"`
}

func NormalizeChatDirectory(c ChatDirectory) (ChatDirectory, error) {
	c.SMBDirectory = strings.Trim(strings.ReplaceAll(strings.TrimSpace(c.SMBDirectory), "\\", "/"), "/")
	for _, part := range strings.Split(c.SMBDirectory, "/") {
		if part == ".." {
			return c, auth.NewHTTPError(400, "聊天目录不能包含上级路径")
		}
	}
	if strings.ContainsAny(c.SMBDirectory, ":\x00") || len(c.SMBDirectory) > 1024 {
		return c, auth.NewHTTPError(400, "请填写 SMB 共享内的相对目录")
	}
	c.SMBDirectory = path.Clean(c.SMBDirectory)
	if c.SMBDirectory == "." {
		c.SMBDirectory = ""
	}
	if c.Enabled && c.SMBDirectory == "" {
		return c, auth.NewHTTPError(400, "请填写聊天日志目录")
	}
	c.FileGlob = strings.TrimSpace(c.FileGlob)
	if c.FileGlob == "" {
		c.FileGlob = "*.txt"
	}
	if strings.ContainsAny(c.FileGlob, "/\\\x00") || len(c.FileGlob) > 128 || !strings.HasSuffix(c.FileGlob, ".txt") {
		return c, auth.NewHTTPError(400, "聊天文件匹配必须是 TXT 文件名规则，例如 *.txt")
	}
	if _, err := filepath.Match(c.FileGlob, "2026-10-23.txt"); err != nil {
		return c, auth.NewHTTPError(400, "文件匹配规则无效")
	}
	return c, nil
}
