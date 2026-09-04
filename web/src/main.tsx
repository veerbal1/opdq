import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "./index.css";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import Login from "./pages/Login.tsx";
import Console from "./pages/Console.tsx";
import Board from "./pages/Board.tsx";
import { AuthProvider, RequireAuth } from "./auth.tsx";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route
            path="/console"
            element={
              <RequireAuth>
                <Console />
              </RequireAuth>
            }
          />
          <Route path="/" element={<Navigate to="/console" replace />} />
          <Route path="/board/:id" element={<Board />} />
          <Route path="*" element={<p>Page not found</p>} />
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  </StrictMode>,
);
