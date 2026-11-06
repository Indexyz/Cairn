import { useState } from "react";
import { NavLink, Outlet } from "react-router-dom";
import {
  ChevronsLeft,
  ChevronsRight,
  LayoutDashboard,
  Settings,
  Sparkles,
  UserRound,
} from "lucide-react";

import { usePrefs } from "@/hooks/use-prefs";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";

const NAV = [
  { to: "/", label: "Projects", icon: LayoutDashboard, end: true },
  { to: "/models", label: "Models", icon: Sparkles, end: false },
];

export function AppShell() {
  const [collapsed, setCollapsed] = useState(false);
  const { actorName, openSettings } = usePrefs();

  return (
    <div className="flex h-full overflow-hidden bg-muted/30">
      <aside
        className={cn(
          "flex shrink-0 flex-col border-r border-sidebar-border bg-sidebar transition-[width] duration-200",
          collapsed ? "w-16" : "w-60",
        )}
      >
        <div className="flex h-14 items-center gap-2 px-3">
          <img src="/favicon.svg" alt="" className="size-8 shrink-0 rounded-lg border bg-background p-1" />
          {!collapsed && (
            <span className="truncate text-sm font-semibold tracking-tight">Cairn</span>
          )}
          <div className="flex-1" />
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={() => setCollapsed((value) => !value)}
            aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
          >
            {collapsed ? <ChevronsRight className="size-4" /> : <ChevronsLeft className="size-4" />}
          </Button>
        </div>

        <nav className="flex flex-1 flex-col gap-1 px-2 pt-2">
          {!collapsed && (
            <p className="px-2 pb-1 text-[11px] font-medium tracking-wide text-muted-foreground">
              Workspace
            </p>
          )}
          {NAV.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.end}
              className={({ isActive }) =>
                cn(
                  "flex h-9 items-center gap-2.5 rounded-md px-2.5 text-sm transition-colors",
                  isActive
                    ? "bg-sidebar-accent font-medium text-sidebar-accent-foreground"
                    : "text-muted-foreground hover:bg-sidebar-accent/60 hover:text-foreground",
                  collapsed && "justify-center px-0",
                )
              }
              title={item.label}
            >
              <item.icon className="size-4 shrink-0" />
              {!collapsed && <span className="truncate">{item.label}</span>}
            </NavLink>
          ))}

          <div className="mt-auto pb-2">
            {!collapsed && (
              <p className="px-2 pb-1 text-[11px] font-medium tracking-wide text-muted-foreground">
                System
              </p>
            )}
            <button
              type="button"
              onClick={openSettings}
              className={cn(
                "flex h-9 w-full items-center gap-2.5 rounded-md px-2.5 text-sm text-muted-foreground transition-colors hover:bg-sidebar-accent/60 hover:text-foreground",
                collapsed && "justify-center px-0",
              )}
              title="Settings"
            >
              <Settings className="size-4 shrink-0" />
              {!collapsed && <span>Settings</span>}
            </button>
          </div>
        </nav>

        <div className="border-t border-sidebar-border p-2">
          <button
            type="button"
            onClick={openSettings}
            className={cn(
              "flex h-10 w-full items-center gap-2 rounded-md px-2 text-left text-sm hover:bg-sidebar-accent/60",
              collapsed && "justify-center px-0",
            )}
            title={actorName}
          >
            <span className="flex size-7 shrink-0 items-center justify-center rounded-full bg-muted">
              <UserRound className="size-4 text-muted-foreground" />
            </span>
            {!collapsed && <span className="truncate text-muted-foreground">{actorName}</span>}
          </button>
        </div>
      </aside>

      <main className="flex min-w-0 flex-1 flex-col overflow-hidden">
        <Outlet />
      </main>
    </div>
  );
}
