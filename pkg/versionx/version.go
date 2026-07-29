package versionx

// InfoSvc 版本信息结构体
type InfoSvc struct {
	Version   string
	GitBranch string
	GitCommit string
	BuildTime string
}

var (
	// VersionInfo 全局版本信息实例
	VersionInfo = &InfoSvc{}
)

// NewInfoSvc 提供一个构造函数用于 wire 注入
func NewInfoSvc() *InfoSvc {
	return VersionInfo
}

// GetVersion 获取版本信息
func (v *InfoSvc) GetVersion() map[string]string {
	return map[string]string{
		"version":    fallbackVersionValue(v.Version),
		"git_branch": fallbackVersionValue(v.GitBranch),
		"git_commit": fallbackVersionValue(v.GitCommit),
		"build_time": fallbackVersionValue(v.BuildTime),
	}
}

func fallbackVersionValue(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
