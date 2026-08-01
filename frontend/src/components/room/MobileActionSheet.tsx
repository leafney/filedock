import { X } from "lucide-react";
import { useTranslation } from "react-i18next";
import type { ReactNode } from "react";

export function MobileActionSheet({ title, open, onClose, children }: { title: string; open: boolean; onClose: () => void; children: ReactNode }) {
  const { t } = useTranslation();
  if (!open) return null;
  return <>
    <button className="file-action-backdrop" type="button" aria-label={t("room.workspace.close")} onClick={onClose} />
    <div className="file-action-sheet" role="menu" aria-label={title}>
      <header><strong>{title}</strong><button type="button" aria-label={t("room.workspace.close")} onClick={onClose}><X aria-hidden="true" /></button></header>
      {children}
    </div>
  </>;
}
