import { fetchEventSource } from "@microsoft/fetch-event-source";
import { useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";

import { currentLanguage } from "../i18n";
import type { Session, StreamEvent } from "../types/domain";

export const streamEventName = "filedock:stream-event";

export interface StreamEventMessage {
  event: StreamEvent;
}

class FatalStreamError extends Error {}

export function useGlobalStream(session: Session | undefined) {
  const queryClient = useQueryClient();

  useEffect(() => {
    if (!session?.userId) {
      return undefined;
    }
    const controller = new AbortController();
    const handleEvent = (event: StreamEvent) => {
      window.dispatchEvent(new CustomEvent<StreamEventMessage>(streamEventName, { detail: { event } }));
      if (event.type === "session.revoked") {
        queryClient.removeQueries({ queryKey: ["session"] });
        queryClient.removeQueries({ queryKey: ["rooms"] });
        return;
      }
      if (event.type.startsWith("room.") || event.type === "user.profile_changed") {
        void queryClient.invalidateQueries({ queryKey: ["rooms"] });
        void queryClient.invalidateQueries({ queryKey: ["room"] });
        if (event.type === "room.join_request_changed") {
          void queryClient.invalidateQueries({ queryKey: ["join-requests"] });
        }
        return;
      }
      if (event.type.startsWith("file.")) {
        if (event.type !== "file.upload_progress" && event.type !== "file.download_progress") {
          void queryClient.invalidateQueries({ queryKey: ["room"] });
        }
        return;
      }
      const known = new Set(["session.revoked", "user.profile_changed"]);
      if (!known.has(event.type)) {
        void queryClient.invalidateQueries({ queryKey: ["session"] });
        void queryClient.invalidateQueries({ queryKey: ["rooms"] });
        void queryClient.invalidateQueries({ queryKey: ["room"] });
      }
    };

    void fetchEventSource("/api/v1/stream", {
      signal: controller.signal,
      credentials: "include",
      openWhenHidden: true,
      headers: { "Accept-Language": currentLanguage(), Accept: "text/event-stream" },
      async onopen(response) {
        if (response.ok && response.headers.get("content-type")?.includes("text/event-stream")) {
          void queryClient.invalidateQueries({ queryKey: ["session"] });
          return;
        }
        if (response.status === 401 || response.status === 403) {
          throw new FatalStreamError("stream authentication failed");
        }
        throw new Error(`stream request failed: ${response.status}`);
      },
      onmessage(message) {
        if (!message.data) {
          return;
        }
        try {
          const event = JSON.parse(message.data) as StreamEvent;
          if (typeof event.type === "string") {
            handleEvent(event);
          }
        } catch {
          // The next REST snapshot remains the source of truth.
        }
      },
      onerror(error) {
        if (error instanceof FatalStreamError) {
          throw error;
        }
        return 5_000;
      },
    }).catch(() => {
      // The component owns the AbortController; a later render reconnects.
    });

    return () => controller.abort();
  }, [queryClient, session?.userId]);
}
