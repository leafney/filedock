package service

type BuildInfo struct {
	Version   string
	Branch    string
	Commit    string
	BuildTime string
}

type VersionSvc struct {
	build BuildInfo
}

func NewVersionSvc(build BuildInfo) *VersionSvc {
	return &VersionSvc{build: build}
}

func (s *VersionSvc) GetVersion() BuildInfo {
	if s == nil {
		return BuildInfo{}
	}
	return s.build
}
