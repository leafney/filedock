import { useQuery } from "@tanstack/react-query";

import { getCurrentSession } from "../services/api";

export function useSessionQuery() {
  return useQuery({ queryKey: ["session"], queryFn: getCurrentSession, retry: false, staleTime: 60_000 });
}
