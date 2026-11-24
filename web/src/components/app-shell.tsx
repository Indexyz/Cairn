import { useMemo, useState } from "react";
import { Link, NavLink, Outlet } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  Bell,
  CalendarClock,
  ChevronsLeft,
  ChevronsRight,
  Languages,
  LayoutDashboard,
  Monitor,
  Moon,
  Plug,
  Settings,
  Sparkles,
  Sun,
  UserRound,
} from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Select,
  SelectItem,
  SelectPopup,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { Tooltip, TooltipPopup, TooltipTrigger } from "@/components/ui/tooltip";
import { usePrefs } from "@/hooks/use-prefs";
import { THEME_MODES, useTheme, type ThemeMode } from "@/hooks/use-theme";
import { SUPPORTED_LOCALES, currentLocale, setLocale, type Locale } from "@/lib/i18n";
import { cn } from "@/lib/utils";

const FAVICON_URL = `${import.meta.env.BASE_URL}favicon.svg`;

const THEME_ICONS: Record<ThemeMode, typeof Sun> = {
  system: Monitor,
  light: Sun,
  dark: Moon,
};

export function AppShell() {
  const [collapsed, setCollapsed] = useState(false);
  const { actorName } = usePrefs();
  const { theme, setTheme, cycleTheme } = useTheme();
  const { t } = useTranslation();

  const nav = [
    { to: "/", label: t("nav.projects"), icon: LayoutDashboard, end: true },
    { to: "/models", label: t("nav.models"), icon: Sparkles, end: false },
    { to: "/mcp", label: t("nav.mcp"), icon: Plug, end: false },
    { to: "/notifications", label: t("nav.notifications"), icon: Bell, end: false },
    { to: "/schedules", label: t("nav.schedules"), icon: CalendarClock, end: false },
  ];

  const localeItems = useMemo(
    () => SUPPORTED_LOCALES.map((locale) => ({ value: locale, label: t(`language.${locale}`) })),
    [t],
  );

  const ActiveThemeIcon = THEME_ICONS[theme];

  return (
    <div className="flex h-full overflow-hidden bg-background">
      <aside
        aria-label={t("nav.workspace")}
        className={cn(
          "relative flex shrink-0 flex-col border-r border-sidebar-border bg-sidebar transition-[width] duration-200",
          collapsed ? "w-15" : "w-58",
        )}
      >
        <div className="flex h-14 items-center gap-2.5 px-3">
          <Link
            to="/"
            className={cn(
              "flex min-w-0 items-center gap-2.5 rounded-lg outline-none focus-visible:ring-2 focus-visible:ring-sidebar-ring",
              collapsed && "mx-auto",
            )}
          >
            <span className="flex size-8 shrink-0 items-center justify-center rounded-lg border bg-card shadow-xs/5">
              <img src={FAVICON_URL} alt="" className="size-5" />
            </span>
            {!collapsed && (
              <span className="truncate font-heading text-sm font-semibold tracking-tight text-foreground">
                Cairn
              </span>
            )}
          </Link>
        </div>

        {/* Kept out of the header flow so it stays reachable at both widths. */}
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant="outline"
                size="icon-xs"
                onClick={() => setCollapsed((value) => !value)}
                aria-label={collapsed ? t("nav.expand") : t("nav.collapse")}
                className="absolute top-4 -right-3 z-20 rounded-full bg-background"
              />
            }
          >
            {collapsed ? (
              <ChevronsRight className="size-3.5" />
            ) : (
              <ChevronsLeft className="size-3.5" />
            )}
          </TooltipTrigger>
          <TooltipPopup side="right">
            {collapsed ? t("nav.expand") : t("nav.collapse")}
          </TooltipPopup>
        </Tooltip>

        <nav aria-label={t("nav.workspace")} className="flex flex-1 flex-col gap-0.5 px-2 pt-2">
          {!collapsed && (
            <p className="px-2.5 pb-1.5 text-[11px] font-medium text-muted-foreground">
              {t("nav.workspace")}
            </p>
          )}
          {nav.map((item) => {
            const link = (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.end}
                className={({ isActive }) =>
                  cn(
                    "flex h-8.5 items-center gap-2.5 rounded-lg px-2.5 text-sm transition-colors outline-none focus-visible:ring-2 focus-visible:ring-sidebar-ring",
                    isActive
                      ? "bg-sidebar-accent font-medium text-sidebar-accent-foreground shadow-[inset_0_0_0_1px_var(--sidebar-border)]"
                      : "text-sidebar-foreground hover:bg-sidebar-accent/70 hover:text-sidebar-accent-foreground",
                    collapsed && "justify-center px-0",
                  )
                }
                title={collapsed ? item.label : undefined}
              >
                <item.icon className="size-4 shrink-0" />
                {!collapsed && <span className="truncate">{item.label}</span>}
              </NavLink>
            );

            if (!collapsed) return link;
            return (
              <Tooltip key={item.to}>
                <TooltipTrigger render={link} />
                <TooltipPopup side="right">{item.label}</TooltipPopup>
              </Tooltip>
            );
          })}

          <div className="mt-auto flex flex-col gap-1 pb-2">
            {!collapsed && (
              <p className="px-2.5 pb-1 text-[11px] font-medium text-muted-foreground">
                {t("nav.system")}
              </p>
            )}
            {(() => {
              const settingsLink = (
                <NavLink
                  to="/settings"
                  className={({ isActive }) =>
                    cn(
                      "flex h-8.5 w-full items-center gap-2.5 rounded-lg px-2.5 text-sm transition-colors outline-none focus-visible:ring-2 focus-visible:ring-sidebar-ring",
                      isActive
                        ? "bg-sidebar-accent font-medium text-sidebar-accent-foreground shadow-[inset_0_0_0_1px_var(--sidebar-border)]"
                        : "text-sidebar-foreground hover:bg-sidebar-accent/70 hover:text-sidebar-accent-foreground",
                      collapsed && "justify-center px-0",
                    )
                  }
                  title={collapsed ? t("nav.settings") : undefined}
                >
                  <Settings className="size-4 shrink-0" />
                  {!collapsed && <span>{t("nav.settings")}</span>}
                </NavLink>
              );

              if (!collapsed) return settingsLink;
              return (
                <Tooltip>
                  <TooltipTrigger render={settingsLink} />
                  <TooltipPopup side="right">{t("nav.settings")}</TooltipPopup>
                </Tooltip>
              );
            })()}

            {!collapsed ? (
              <div className="mt-1 flex flex-col gap-1.5 px-0.5">
                <Select
                  items={localeItems}
                  value={currentLocale()}
                  onValueChange={(value) => {
                    if (value) setLocale(value as Locale);
                  }}
                >
                  <SelectTrigger size="sm" aria-label={t("settings.language")}>
                    <span className="flex min-w-0 items-center gap-2">
                      <Languages className="size-3.5 shrink-0 text-muted-foreground" />
                      <SelectValue />
                    </span>
                  </SelectTrigger>
                  <SelectPopup>
                    {localeItems.map((item) => (
                      <SelectItem key={item.value} value={item.value}>
                        {item.label}
                      </SelectItem>
                    ))}
                  </SelectPopup>
                </Select>

                <div
                  role="group"
                  aria-label={t("theme.label")}
                  className="grid grid-cols-3 gap-0.5 rounded-lg bg-muted p-0.5"
                >
                  {THEME_MODES.map((mode) => {
                    const Icon = THEME_ICONS[mode];
                    const active = theme === mode;
                    return (
                      <button
                        key={mode}
                        type="button"
                        onClick={() => setTheme(mode)}
                        aria-pressed={active}
                        className={cn(
                          "flex h-6.5 items-center justify-center gap-1 rounded-md px-1.5 text-xs font-medium transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring",
                          active
                            ? "bg-background text-foreground shadow-xs/5"
                            : "text-muted-foreground hover:text-foreground",
                        )}
                        title={t(`theme.${mode}`)}
                      >
                        <Icon className="size-3.5 shrink-0" />
                        <span className="truncate text-[11px]">{t(`theme.${mode}`)}</span>
                      </button>
                    );
                  })}
                </div>
              </div>
            ) : (
              <>
                <Tooltip>
                  <TooltipTrigger
                    render={
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        className="mx-auto text-sidebar-foreground"
                        aria-label={t("settings.language")}
                        onClick={() => {
                          const index = SUPPORTED_LOCALES.indexOf(currentLocale());
                          setLocale(SUPPORTED_LOCALES[(index + 1) % SUPPORTED_LOCALES.length]);
                        }}
                      />
                    }
                  >
                    <Languages className="size-4" />
                  </TooltipTrigger>
                  <TooltipPopup side="right">{t("settings.language")}</TooltipPopup>
                </Tooltip>

                <Tooltip>
                  <TooltipTrigger
                    render={
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        className="mx-auto text-sidebar-foreground"
                        aria-label={t("theme.label")}
                        onClick={cycleTheme}
                      />
                    }
                  >
                    <ActiveThemeIcon className="size-4" />
                  </TooltipTrigger>
                  <TooltipPopup side="right">
                    {t("theme.label")}: {t(`theme.${theme}`)}
                  </TooltipPopup>
                </Tooltip>
              </>
            )}
          </div>
        </nav>

        <Separator className="bg-sidebar-border" />

        <div className="p-2">
          <Link
            to="/settings"
            className={cn(
              "flex h-9.5 w-full items-center gap-2.5 rounded-lg px-2 text-left text-sm transition-colors outline-none hover:bg-sidebar-accent/70 focus-visible:ring-2 focus-visible:ring-sidebar-ring",
              collapsed && "justify-center px-0",
            )}
            title={actorName}
          >
            <span className="flex size-7 shrink-0 items-center justify-center rounded-full border bg-card text-muted-foreground shadow-xs/5">
              <UserRound className="size-3.5" />
            </span>
            {!collapsed && (
              <span className="truncate text-xs font-medium text-foreground">{actorName}</span>
            )}
          </Link>
        </div>
      </aside>

      <main className="flex min-w-0 flex-1 flex-col overflow-hidden bg-background">
        <Outlet />
      </main>
    </div>
  );
}
