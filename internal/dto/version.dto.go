package dto

type VersionDTO struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	GitBranch string `json:"git_branch"`
	GitCommit string `json:"git_commit"`
	BuildTime string `json:"build_time"`
}
