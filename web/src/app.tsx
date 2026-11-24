import { BrowserRouter, Route, Routes } from "react-router-dom";

import { AppShell } from "@/components/app-shell";
import { TooltipProvider } from "@/components/ui/tooltip";
import { PrefsProvider } from "@/hooks/use-prefs";
import { ThemeProvider } from "@/hooks/use-theme";
import { ToastProvider } from "@/hooks/use-toast";
import { MCPPage } from "@/pages/mcp";
import { ModelsPage } from "@/pages/models";
import { NotificationsPage } from "@/pages/notifications";
import { ProjectPage } from "@/pages/project";
import { ProjectsPage } from "@/pages/projects";
import { SchedulesPage } from "@/pages/schedules";
import { SettingsPage } from "@/pages/settings";

export function App() {
  return (
    <ThemeProvider>
      <TooltipProvider delay={300}>
        <ToastProvider>
          <PrefsProvider>
            <BrowserRouter>
              <Routes>
                <Route element={<AppShell />}>
                  <Route index element={<ProjectsPage />} />
                  <Route path="models" element={<ModelsPage />} />
                  <Route path="mcp" element={<MCPPage />} />
                  <Route path="notifications" element={<NotificationsPage />} />
                  <Route path="schedules" element={<SchedulesPage />} />
                  <Route path="settings" element={<SettingsPage />} />
                  <Route path="projects/:projectId" element={<ProjectPage />} />
                </Route>
              </Routes>
            </BrowserRouter>
          </PrefsProvider>
        </ToastProvider>
      </TooltipProvider>
    </ThemeProvider>
  );
}
