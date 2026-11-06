import * as React from "react";

import { useLocalPrefs, type LocalPrefs } from "@/hooks/use-local-prefs";

interface PrefsContextValue {
  prefs: LocalPrefs;
  save: (next: LocalPrefs) => void;
  actorName: string;
  openSettings: () => void;
}

const PrefsContext = React.createContext<PrefsContextValue | null>(null);

export function PrefsProvider({ children }: { children: React.ReactNode }) {
  const { prefs, save, actorName } = useLocalPrefs();
  const [settingsOpen, setSettingsOpen] = React.useState(false);

  const value = React.useMemo(
    () => ({ prefs, save, actorName, openSettings: () => setSettingsOpen(true) }),
    [prefs, save, actorName],
  );

  return (
    <PrefsContext.Provider value={value}>
      {children}
      <SettingsDialog open={settingsOpen} onOpenChange={setSettingsOpen} />
    </PrefsContext.Provider>
  );
}

export function usePrefs() {
  const context = React.useContext(PrefsContext);
  if (!context) throw new Error("usePrefs must be used inside PrefsProvider");
  return context;
}

import { SettingsDialog } from "@/components/settings-dialog";
