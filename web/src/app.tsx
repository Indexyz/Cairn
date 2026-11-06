import { HashRouter, Route, Routes } from "react-router-dom";

import { AppShell } from "@/components/app-shell";
import { PrefsProvider } from "@/hooks/use-prefs";
import { ToastProvider } from "@/hooks/use-toast";
import { ModelsPage } from "@/pages/models";
import { ProjectPage } from "@/pages/project";
import { ProjectsPage } from "@/pages/projects";

export function App() {
  return (
    <ToastProvider>
      <PrefsProvider>
        <HashRouter>
          <Routes>
            <Route element={<AppShell />}>
              <Route index element={<ProjectsPage />} />
              <Route path="models" element={<ModelsPage />} />
              <Route path="projects/:projectId" element={<ProjectPage />} />
            </Route>
          </Routes>
        </HashRouter>
      </PrefsProvider>
    </ToastProvider>
  );
}
