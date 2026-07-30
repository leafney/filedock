import { Route, Routes } from "react-router-dom";

import { useGlobalStream } from "../hooks/use-stream";
import { useSessionQuery } from "../hooks/use-session";
import { isUnauthorized } from "../components/common";
import { HomePage } from "../pages/HomePage";
import { RoomPage } from "../pages/RoomPage";

export function App() {
  const sessionQuery = useSessionQuery();
  const session = sessionQuery.error && isUnauthorized(sessionQuery.error) ? undefined : sessionQuery.data;
  useGlobalStream(session);
  return (
    <Routes>
      <Route path="/" element={<HomePage sessionQuery={sessionQuery} />} />
      <Route path="/rooms/:code" element={<RoomPage sessionQuery={sessionQuery} />} />
      <Route path="*" element={<HomePage sessionQuery={sessionQuery} />} />
    </Routes>
  );
}
