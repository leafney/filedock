import { useInfiniteQuery } from "@tanstack/react-query";
import { useMemo } from "react";

import { listNotifications } from "../services/api";
import type { Session } from "../types/domain";

export const notificationQueryKey = ["notifications"] as const;

export function useNotifications(session: Session | undefined) {
  const query = useInfiniteQuery({
    queryKey: notificationQueryKey,
    queryFn: ({ pageParam }) => listNotifications(pageParam),
    initialPageParam: "",
    getNextPageParam: (lastPage) => lastPage.nextCursor || undefined,
    enabled: Boolean(session?.userId),
    retry: false,
  });
  const items = useMemo(() => query.data?.pages.flatMap((page) => page.items) ?? [], [query.data?.pages]);
  const totalCount = query.data?.pages[0]?.totalCount ?? 0;
  return { ...query, items, totalCount };
}
