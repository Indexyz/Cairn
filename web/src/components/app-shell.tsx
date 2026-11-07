import { useState } from "react";
import { NavLink, Outlet } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  ChevronsLeft,
  ChevronsRight,
  LayoutDashboard,
  Languages,
  Settings,
  Sparkles,
  UserRound,
} from "lucide-react";

import { usePrefs } from "@/hooks/use-prefs";
import { SUPPORTED_LOCALES, currentLocale, setLocale, type Locale } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

const FAVICON_URL = `${import.meta.env.BASE_URL}favicon.svg`;

export function AppShell() {
  const [collapsed, setCollapsed] = useState(false);
  const { actorName, openSettings } = usePrefs();
  const { t } = useTranslation();

  const nav = [
    { to: "/", label: t("nav.projects"), icon: LayoutDashboard, end: true },
    { to: "/models", label: t("nav.models"), icon: Sparkles, end: false },
  ];

  return (
    <div className="flex h-full overflow-hidden bg-muted/30">
      <aside
        className={cn(
          "flex shrink-0 flex-col border-r border-sidebar-border bg-sidebar transition-[width] duration-200",
          collapsed ? "w-16" : "w-60",
        )}
      >
        <div className="flex h-14 items-center gap-2 px-3">
          <img src={FAVICON_URL} alt="" className="size-8 shrink-0 rounded-lg border bg-background p-1" />
          {!collapsed && (
            <span className="truncate text-sm font-semibold tracking-tight">Cairn</span>
          )}
          <div className="flex-1" />
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={() => setCollapsed((value) => !value)}
            aria-label={collapsed ? t("nav.expand") : t("nav.collapse")}
          >
            {collapsed ? <ChevronsRight className="size-4" /> : <ChevronsLeft className="size-4" />}
          </Button>
        </div>

        <nav className="flex flex-1 flex-col gap-1 px-2 pt-2">
              {!collapsed && (
                <p className="px-2 pb-1 text-[11px] font-medium tracking-wide text-muted-foreground">
                  {t("nav.workspace")}
                </p>
              )}
              {nav.map((item) => (
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
                  {t("nav.system")}
                </p>
              )}
              <button
                type="button"
                onClick={openSettings}
                className={cn(
                  "flex h-9 w-full items-center gap-2.5 rounded-md px-2.5 text-sm text-muted-foreground transition-colors hover:bg-sidebar-accent/60 hover:text-foreground",
                  collapsed && "justify-center px-0",
                )}
                title={t("nav.settings")}
              >
                <Settings className="size-4 shrink-0" />
                {!collapsed && <span>{t("nav.settings")}</span>}
              </button>

              {!collapsed ? (
                <Select value={currentLocale()} onValueChange={(value) => setLocale(value as Locale)}>
                  <SelectTrigger className="mt-1 h-8 w-full border-dashed text-xs text-muted-foreground">
                    <Languages className="size-3.5 shrink-0" />
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {SUPPORTED_LOCALES.map((locale) => (
                      <SelectItem key={locale} value={locale}>
                        {t(`language.${locale}`)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              ) : (
                <button
                  type="button"
                  onClick={() => {
                    const index = SUPPORTED_LOCALES.indexOf(currentLocale());
                    setLocale(SUPPORTED_LOCALES[(index + 1) % SUPPORTED_LOCALES.length]);
                  }}
                  className="mt-1 flex h-9 w-full items-center justify-center rounded-md text-muted-foreground hover:bg-sidebar-accent/60"
                  title={t("settings.language")}
                >
                  <Languages className="size-4" />
                </button>
              )}
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
