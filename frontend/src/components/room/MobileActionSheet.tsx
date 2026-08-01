import { X } from "lucide-react";
import { useTranslation } from "react-i18next";
import { useEffect, useRef, type ReactNode, type RefObject } from "react";

export function MobileActionSheet({ title, open, onClose, triggerRef, children }: { title: string; open: boolean; onClose: () => void; triggerRef?: RefObject<HTMLButtonElement>; children: ReactNode }) {
  const { t } = useTranslation();
  const panelRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!open) return undefined;
    panelRef.current?.querySelector<HTMLElement>("[role='menuitem']")?.focus();
    return () => triggerRef?.current?.focus();
  }, [open, triggerRef]);
  if (!open) return null;
  return <>
    <button className="file-action-backdrop" type="button" aria-label={t("room.workspace.close")} onClick={onClose} />
    <div ref={panelRef} className="file-action-sheet" role="menu" aria-label={title} onKeyDown={(event) => {
      const items = Array.from(event.currentTarget.querySelectorAll<HTMLElement>("[role='menuitem']"));
      const index = items.indexOf(document.activeElement as HTMLElement);
      if (event.key === "Escape") { event.preventDefault(); onClose(); return; }
      if (index < 0 || !["ArrowDown", "ArrowUp", "Home", "End"].includes(event.key)) return;
      event.preventDefault();
      const next = event.key === "Home" ? 0 : event.key === "End" ? items.length - 1 : event.key === "ArrowDown" ? (index + 1) % items.length : (index - 1 + items.length) % items.length;
      items[next]?.focus();
    }}>
      <header><strong>{title}</strong><button type="button" aria-label={t("room.workspace.close")} onClick={onClose}><X aria-hidden="true" /></button></header>
      {children}
    </div>
  </>;
}
