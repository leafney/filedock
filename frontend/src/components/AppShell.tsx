import type { ReactNode } from "react";

import { AppFooter } from "./AppFooter";
import { GlobalHeader, type RoomHeaderActions } from "./GlobalHeader";
import type { Session } from "../types/domain";

interface Props {
  variant: "home" | "room";
  session?: Session;
  fallbackName?: string;
  headerCenter?: ReactNode;
  headerActions?: ReactNode;
  onOpenProfile?: () => void;
  onShare?: () => void;
  roomActions?: RoomHeaderActions;
  footerOverlay?: ReactNode;
  children: ReactNode;
}

/**
 * Shared page frame. Pages provide only their content and optional contextual
 * header/footer slots; the public shell remains stable while routes change.
 */
export function AppShell({
  variant,
  session,
  fallbackName,
  headerCenter,
  headerActions,
  onOpenProfile,
  onShare,
  roomActions,
  footerOverlay,
  children,
}: Props) {
  return (
    <div className={`app-shell ${variant}-page`}>
      <GlobalHeader
        variant={variant}
        session={session}
        fallbackName={fallbackName}
        center={headerCenter}
        extraActions={headerActions}
        onOpenProfile={onOpenProfile ?? (() => undefined)}
        onShare={onShare}
        roomActions={roomActions}
      />
      <main className="app-shell-main">{children}</main>
      <AppFooter />
      {footerOverlay && <div className="app-shell-footer-overlay">{footerOverlay}</div>}
    </div>
  );
}
