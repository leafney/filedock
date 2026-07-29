export interface VersionResponse {
  status: string;
  version: string;
  git_branch: string;
  git_commit: string;
  build_time: string;
}
