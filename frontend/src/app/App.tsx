import { Route, Routes } from "react-router-dom";

import { useGlobalStream } from "../hooks/use-stream";
import { useSessionQuery } from "../hooks/use-session";
import { HomePage } from "../pages/HomePage";
import { RoomPage } from "../pages/RoomPage";

export function App() {
  const sessionQuery = useSessionQuery();
  useGlobalStream(sessionQuery.data);
  return (
    <Routes>
      <Route path="/" element={<HomePage sessionQuery={sessionQuery} />} />
      <Route path="/rooms/:code" element={<RoomPage sessionQuery={sessionQuery} />} />
      <Route path="*" element={<HomePage sessionQuery={sessionQuery} />} />
    </Routes>
  );
}
