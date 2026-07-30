import { InfoCircleOutlined } from "@ant-design/icons";
import { Tooltip } from "antd";
import { useQuery } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";

import { getVersion } from "../services/version";
import { getVersionDetails } from "../utils/version";

export function AppFooter() {
  const { t } = useTranslation();
  const versionQuery = useQuery({ queryKey: ["version"], queryFn: getVersion, retry: false, staleTime: 5 * 60_000 });
  const details = versionQuery.data ? getVersionDetails(versionQuery.data) : { label: "dev", version: "dev", branch: "-", commit: "-", buildTime: "-" };
  const content = (
    <div className="app-footer-tooltip">
      <div>{t("home.fields.version")}: {details.version}</div>
      <div>{t("home.fields.branch")}: {details.branch}</div>
      <div>{t("home.fields.commit")}: {details.commit}</div>
      <div>{t("home.fields.buildTime")}: {details.buildTime}</div>
    </div>
  );
  return (
    <footer className="app-footer">
      <Tooltip title={content} trigger={["hover", "focus", "click"]} placement="top">
        <button className="app-footer-trigger" type="button" aria-label={t("home.versionDetails")}>
          <InfoCircleOutlined aria-hidden="true" /> <span>{details.label}</span>
        </button>
      </Tooltip>
    </footer>
  );
}
