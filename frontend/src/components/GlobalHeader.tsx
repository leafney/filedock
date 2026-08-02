import { BellOutlined, DownOutlined, GlobalOutlined, ShareAltOutlined, SettingOutlined } from "@ant-design/icons";
import type { MenuProps } from "antd";
import { Avatar, Badge, Button, Dropdown } from "antd";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";

import type { Session } from "../types/domain";
import { getAvatarInitial, getStableAvatarColor } from "../utils/avatar";
import { normalizeLanguage } from "../i18n";

export interface RoomHeaderActions {
  role: "owner" | "member";
  canExtend?: boolean;
  onExtend?: () => void;
  onDissolve?: () => void;
  onLeave?: () => void;
}

interface Props {
  variant: "home" | "room";
  session?: Session;
  fallbackName?: string;
  center?: ReactNode;
  pendingCount?: number;
  notificationDisabled?: boolean;
  onOpenNotifications: () => void;
  onOpenProfile: () => void;
  onShare?: () => void;
  extraActions?: ReactNode;
  roomActions?: RoomHeaderActions;
}

export function GlobalHeader({ variant, session, fallbackName = "?", center, pendingCount = 0, notificationDisabled = false, onOpenNotifications, onOpenProfile, onShare, extraActions, roomActions }: Props) {
  const { t, i18n } = useTranslation();
  const roomVariant = variant === "room";
  const displayName = session?.displayName || fallbackName || "?";
  const language = normalizeLanguage(i18n.resolvedLanguage ?? i18n.language) ?? "zh-CN";
  const roomItems: NonNullable<MenuProps["items"]> = [];
  if (roomActions) {
    const children: NonNullable<MenuProps["items"]> = [];
    if (roomActions.role === "owner") {
      if (roomActions.canExtend && roomActions.onExtend) children.push({ key: "room-extend", label: t("room.extend") });
      if (roomActions.onDissolve) children.push({ key: "room-dissolve", danger: true, label: t("room.dissolve") });
    } else if (roomActions.onLeave) {
      children.push({ key: "room-leave", label: t("room.leave") });
    }
    if (children.length > 0) roomItems.push({ key: "room-actions", icon: <SettingOutlined />, label: t("room.workspace.roomActions"), children });
  }
  const menuItems: MenuProps["items"] = [
    ...(session ? [{ key: "profile", icon: <SettingOutlined />, label: t("home.profile") }] : []),
    ...roomItems,
    { key: "language", icon: <GlobalOutlined />, label: language === "zh-CN" ? t("language.en") : t("language.zhCN") },
  ];
  const onMenuClick: MenuProps["onClick"] = ({ key }) => {
    if (key === "profile") { onOpenProfile(); return; }
    if (key === "language") { void i18n.changeLanguage(language === "zh-CN" ? "en" : "zh-CN"); return; }
    if (key === "room-extend") { roomActions?.onExtend?.(); return; }
    if (key === "room-dissolve") { roomActions?.onDissolve?.(); return; }
    if (key === "room-leave") roomActions?.onLeave?.();
  };
  const iconClassName = roomVariant ? "room-icon-button" : "home-icon-button";
  const userClassName = roomVariant ? "room-user-button" : "home-user-button";
  const actionClassName = roomVariant ? "room-topbar-actions" : "home-header-actions";
  const brandClassName = roomVariant ? "room-topbar-leading global-header-brand" : "home-brand global-header-brand";
  const brandMarkClassName = roomVariant ? "room-brand-mark" : "home-brand-mark";

  return <header className={roomVariant ? "room-topbar" : "home-header"}>
    <Link className={brandClassName} to="/" aria-label={t("brand.homeLink")}>
      <span className={brandMarkClassName} aria-hidden="true">FD</span>
      <div><div className="home-brand-name">FileDock</div><div className="home-brand-subtitle">{t("brand.description")}</div></div>
    </Link>
    {center}
    <div className={actionClassName}>
      {extraActions}
      {onShare && <Button className={iconClassName} type="text" shape="circle" icon={<ShareAltOutlined />} aria-label={t("room.workspace.shareRoom")} onClick={onShare} />}
      <Badge count={session && !notificationDisabled ? pendingCount : 0} size="small" overflowCount={99}>
        <Button className={iconClassName} type="text" shape="circle" icon={<BellOutlined />} aria-label={t("room.notifications")} disabled={!session || notificationDisabled} onClick={onOpenNotifications} />
      </Badge>
      <Dropdown menu={{ items: menuItems, onClick: onMenuClick }} trigger={["click"]} placement="bottomRight">
        <button type="button" className={userClassName} aria-label={session ? t("home.profile") : t("language.label")}>
          <Avatar size={34} style={{ backgroundColor: getStableAvatarColor(displayName) }}>{getAvatarInitial(displayName)}</Avatar>
          <span className={roomVariant ? "room-user-name" : "home-user-name"}>{displayName}</span>
          <DownOutlined className={roomVariant ? "room-user-chevron" : "home-user-chevron"} />
        </button>
      </Dropdown>
    </div>
  </header>;
}
