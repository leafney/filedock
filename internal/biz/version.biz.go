package biz

import (
	"fmt"

	"github.com/leafney/filedock/internal/dto"
	"github.com/leafney/filedock/internal/service"
)

type VersionBiz struct {
	version *service.VersionSvc
}

func NewVersionBiz(version *service.VersionSvc) (*VersionBiz, error) {
	if version == nil {
		return nil, fmt.Errorf("version service is required")
	}
	return &VersionBiz{version: version}, nil
}

func (b *VersionBiz) GetVersion() dto.VersionDTO {
	info := b.version.GetVersion()
	return dto.VersionDTO{
		Status:    "ok",
		Version:   info.Version,
		GitBranch: info.Branch,
		GitCommit: info.Commit,
		BuildTime: info.BuildTime,
	}
}
