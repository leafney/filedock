import { InfoCircleOutlined } from "@ant-design/icons";
import { message, Tooltip } from "antd";
import { useQuery } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { getVersion } from "../services/version";
import { getVersionDetails } from "../utils/version";
import { copyText } from "../utils/clipboard";

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
  const details = versionQuery.data ? getVersionDetails(versionQuery.data) : { label: "dev", version: undefined, branch: undefined, commit: undefined, shortCommit: undefined, buildTime: undefined };
  const displayValue = details.version ?? details.shortCommit ?? "dev";
  const content = (
    <div className="app-footer-tooltip">
      {details.version && <div><span>{t("home.fields.version")}:</span><strong>{details.version}</strong></div>}
      {details.shortCommit && <div><span>{t("home.fields.commit")}:</span><strong>{details.shortCommit}</strong></div>}
      {details.buildTime && <div><span>{t("home.fields.compileTime")}:</span><strong>{details.buildTime}</strong></div>}
    </div>
  );
  const copyCommit = async () => {
    if (!details.shortCommit) {
      message.warning(t("home.versionUnavailable"));
      return;
    }
    if (await copyText(details.shortCommit)) message.success(t("home.versionCopied"));
    else message.error(t("home.versionCopyFailed"));
  };
  const trigger = <button className="app-footer-trigger" type="button" aria-label={t("home.versionDetails")} onClick={() => void copyCommit()}>
    <InfoCircleOutlined aria-hidden="true" /> <span>{displayValue}</span>
  </button>;

  return (
    <footer className="app-footer">
      <div className="app-footer-inner">
        {mobile ? <span className="app-footer-mobile-trigger">{trigger}</span> : <Tooltip title={content} trigger={["hover", "focus"]} placement="top">{trigger}</Tooltip>}
      </div>
    </footer>
  );
}
