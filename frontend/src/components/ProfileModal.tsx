import { DeleteOutlined, EditOutlined } from "@ant-design/icons";
import { Alert, Avatar, Button, Form, Input, Modal, Space, Tooltip, message } from "antd";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Dices } from "lucide-react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { getApiErrorMessage } from "../lib/api-error";
import { getRandomNickname, updateSession } from "../services/api";
import { getVersion } from "../services/version";
import type { RoomSummary, Session } from "../types/domain";
import { getAvatarInitial, getStableAvatarColor } from "../utils/avatar";
import { getVersionDetails } from "../utils/version";

function formatRegistrationDate(timestamp: number) {
  if (!timestamp) return "-";
  const date = new Date(timestamp * 1000);
  if (!Number.isFinite(date.getTime())) return "-";
  const pad = (value: number) => String(value).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
}

export interface ProfileModalProps {
  open: boolean;
  onClose: () => void;
  session: Session;
  ownerRoom?: RoomSummary;
  onReset: () => void;
}

export function ProfileModal({ open, onClose, session, ownerRoom, onReset }: ProfileModalProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const versionQuery = useQuery({ queryKey: ["version"], queryFn: getVersion, retry: false, staleTime: 5 * 60_000 });
  const [name, setName] = useState(session.displayName);
  const update = useMutation({
    mutationFn: () => updateSession(name.trim()),
    onSuccess: (next) => {
      queryClient.setQueryData(["session"], next);
      void queryClient.invalidateQueries({ queryKey: ["rooms"] });
      message.success(t("home.saveName"));
    },
  });
  const randomNickname = useMutation({ mutationFn: getRandomNickname, onMutate: () => update.reset(), onSuccess: (candidate) => setName(candidate.displayName) });

  useEffect(() => {
    if (!open) return;
    setName(session.displayName);
    update.reset();
    randomNickname.reset();
  }, [open, session.displayName]);

  const resetDisabled = Boolean(ownerRoom && (ownerRoom.status === "active" || ownerRoom.status === "destroying"));
  const version = versionQuery.data ? getVersionDetails(versionQuery.data) : undefined;
  const profileError = randomNickname.error ?? update.error;

  return <Modal title={t("home.profile")} open={open} onCancel={onClose} footer={null} destroyOnHidden width={440}>
    <div className="home-profile">
      <Avatar size={72} style={{ backgroundColor: getStableAvatarColor(session.displayName), fontSize: 28 }}>{getAvatarInitial(session.displayName)}</Avatar>
      <Form layout="vertical" className="home-profile-form">
        <Form.Item label={t("session.displayName")}>
          <div className="home-profile-name-editor">
            <Space.Compact block>
              <Input value={name} maxLength={20} onChange={(event) => { setName(event.target.value); update.reset(); randomNickname.reset(); }} />
              <Tooltip title={t("session.randomize")}><Button aria-label={t("session.randomize")} icon={<Dices aria-hidden="true" size={16} />} loading={randomNickname.isPending} onClick={() => randomNickname.mutate()} /></Tooltip>
            </Space.Compact>
            <Button type="primary" block icon={<EditOutlined />} loading={update.isPending} disabled={!name.trim()} onClick={() => { randomNickname.reset(); update.mutate(); }}>{t("home.saveName")}</Button>
          </div>
        </Form.Item>
        {profileError && <Alert className="home-alert" type="error" showIcon title={getApiErrorMessage(profileError, t)} />}
        <dl className="home-profile-details"><dt>{t("home.registeredAt")}</dt><dd>{formatRegistrationDate(session.createdAt)}</dd></dl>
        <h3 className="home-profile-version-title">{t("home.serviceInfo")}</h3>
        <dl className="home-profile-details">
          <dt>{t("home.fields.status")}</dt><dd>{versionQuery.data?.status ?? "-"}</dd>
          <dt>{t("home.fields.version")}</dt><dd>{version?.version ?? "-"}</dd>
          <dt>{t("home.fields.branch")}</dt><dd>{version?.branch ?? "-"}</dd>
          <dt>{t("home.fields.commit")}</dt><dd>{version?.commit ?? "-"}</dd>
          <dt>{t("home.fields.buildTime")}</dt><dd>{version?.buildTime ?? "-"}</dd>
        </dl>
        <Tooltip title={resetDisabled ? t("home.resetBlocked") : undefined}>
          <Button danger block icon={<DeleteOutlined />} disabled={resetDisabled} onClick={onReset}>{t("session.reset")}</Button>
        </Tooltip>
      </Form>
    </div>
  </Modal>;
}
