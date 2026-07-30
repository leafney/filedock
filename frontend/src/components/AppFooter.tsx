import { InfoCircleOutlined } from "@ant-design/icons";
import { Tooltip } from "antd";
import { useQuery } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { getVersion } from "../services/version";
import { getVersionDetails } from "../utils/version";

const MOBILE_FOOTER_QUERY = "(max-width: 560px)";

function useMobileFooter() {
  const [mobile, setMobile] = useState(() => window.matchMedia(MOBILE_FOOTER_QUERY).matches);
  useEffect(() => {
    const media = window.matchMedia(MOBILE_FOOTER_QUERY);
    const update = () => setMobile(media.matches);
    update();
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, []);
  return mobile;
}

export function AppFooter() {
  const { t } = useTranslation();
  const mobile = useMobileFooter();
  const versionQuery = useQuery({ queryKey: ["version"], queryFn: getVersion, retry: false, staleTime: 5 * 60_000 });
  const details = versionQuery.data ? getVersionDetails(versionQuery.data) : { label: "dev", version: "dev", branch: "-", commit: "-", buildTime: "-" };
  const content = (
    <div className="app-footer-tooltip">
      <div>{t("home.fields.version")}: {details.version}</div>
      <div>{t("home.fields.buildTime")}: {details.buildTime}</div>
    </div>
  );

  return (
    <footer className="app-footer">
      {mobile ? (
        <span className="app-footer-text">{details.label}</span>
      ) : (
        <Tooltip title={content} trigger={["hover", "focus"]} placement="right">
          <button className="app-footer-trigger" type="button" aria-label={t("home.versionDetails")}>
            <InfoCircleOutlined aria-hidden="true" /> <span>{details.label}</span>
          </button>
        </Tooltip>
      )}
    </footer>
  );
}
