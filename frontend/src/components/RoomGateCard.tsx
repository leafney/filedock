import type { ReactNode } from "react";
import { useId } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router-dom";

export type RoomGateTone = "info" | "primary" | "pin" | "waiting" | "error" | "unavailable";

interface RoomGateAction {
  key: string;
  label: string;
  onClick?: () => void;
  icon?: ReactNode;
  disabled?: boolean;
  loading?: boolean;
  danger?: boolean;
  href?: string;
}

interface Props {
  tone: RoomGateTone;
  icon: ReactNode;
  label: string;
  title: string;
  description?: string;
  roomCode?: string;
  roomTitle?: string;
  children?: ReactNode;
  actions?: RoomGateAction[];
  live?: boolean;
  error?: ReactNode;
}

export function RoomGateCard({ tone, icon, label, title, description, roomCode, roomTitle, children, actions = [], live = false, error }: Props) {
  const { t } = useTranslation();
  const titleId = useId();
  const hasRoomInfo = Boolean(roomCode || roomTitle);
  return (
    <section className={`room-gate-card room-gate-card-${tone}`} aria-labelledby={titleId} aria-live={live ? "polite" : undefined} role="region">
      <div className="room-gate-status-icon" aria-hidden="true">{icon}</div>
      <span className="room-gate-status-label">{label}</span>
      <h1 id={titleId}>{title}</h1>
      {description && <p className="room-gate-description">{description}</p>}
      {hasRoomInfo && <div className="room-gate-room-info">
        {roomTitle && <strong className="room-gate-room-title">{roomTitle}</strong>}
        {roomCode && <div className="room-gate-room-code"><span>{t("room.code")}</span><b>{roomCode}</b></div>}
      </div>}
      {error && <div className="room-gate-error" role="alert">{error}</div>}
      {children}
      <div className={`room-gate-actions${actions.length === 0 ? " is-single" : ""}`}>
        <Link className="room-gate-action room-gate-action-secondary" to="/" reloadDocument>{t("room.backHome")}</Link>
        {actions.map((action) => action.href ? (
          <a key={action.key} className={`room-gate-action${action.danger ? " is-danger" : ""}`} href={action.href}>{action.label}</a>
        ) : (
          <button key={action.key} className={`room-gate-action room-gate-action-primary${action.danger ? " is-danger" : ""}`} type="button" disabled={action.disabled || action.loading} onClick={action.onClick}>
            {action.loading && <span className="room-gate-action-spinner" aria-hidden="true" />}
            {action.icon}
            {action.label}
          </button>
        ))}
      </div>
    </section>
  );
}

export type { RoomGateAction };
