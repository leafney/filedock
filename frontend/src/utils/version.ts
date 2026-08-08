import type { VersionResponse } from "../types/version";

function valid(value: string) {
  return value !== "" && value !== "-" && value !== "unknown" && value !== "dev";
}

export function getVersionLabel(version: VersionResponse) {
  if (valid(version.version) && !version.version.includes("-g")) return version.version;
  if (valid(version.git_commit)) return version.git_commit.slice(0, 7);
  return "dev";
}

export function getVersionDetails(version: VersionResponse) {
  const versionLabel = valid(version.version) && !version.version.includes("-g") ? version.version : undefined;
  const commit = valid(version.git_commit) ? version.git_commit : undefined;
  return {
    label: getVersionLabel(version),
    version: versionLabel,
    branch: valid(version.git_branch) ? version.git_branch : undefined,
    commit,
    shortCommit: commit?.slice(0, 7),
    buildTime: valid(version.build_time) ? version.build_time : undefined,
  };
}
