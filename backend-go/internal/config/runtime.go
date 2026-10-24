package config

import "fmt"

// RuntimeParameters are persisted business tuning values, loaded once at startup.
type RuntimeParameters struct {
	LiveCacheSeconds int `json:"liveCacheSeconds"`
	MaxFilesPerRun   int `json:"maxFilesPerRun"`
	XrayRetainedJobs int `json:"xrayRetainedJobs"`
}

func (c Config) RuntimeParameters() RuntimeParameters {
	return RuntimeParameters{c.LiveLogSnapshotTTLSeconds, c.MaxFilesPerRun, c.XrayRetainedJobs}
}

func (p RuntimeParameters) Validate() error {
	if p.LiveCacheSeconds < 10 || p.LiveCacheSeconds > 600 {
		return fmt.Errorf("当天日志缓存时长必须为 10～600 秒")
	}
	if p.MaxFilesPerRun < 1 || p.MaxFilesPerRun > 100000 {
		return fmt.Errorf("每轮文件处理上限必须为 1～100000")
	}
	if p.XrayRetainedJobs < 1 || p.XrayRetainedJobs > 20 {
		return fmt.Errorf("矿透任务保留数量必须为 1～20")
	}
	return nil
}

func (c Config) WithRuntime(p RuntimeParameters) Config {
	c.LiveLogSnapshotTTLSeconds = p.LiveCacheSeconds
	c.MaxFilesPerRun = p.MaxFilesPerRun
	c.XrayRetainedJobs = p.XrayRetainedJobs
	return c
}
