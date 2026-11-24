import * as React from "react";

import { ToastProvider as CossToastProvider, toastManager } from "@/components/ui/toast";

export type ToastType = "info" | "success" | "warning" | "error";

interface ToastContextValue {
  show: (message: string, type?: ToastType) => void;
}

const ToastContext = React.createContext<ToastContextValue | null>(null);

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const show = React.useCallback((message: string, type: ToastType = "info") => {
    toastManager.add({
      title: message,
      type,
      timeout: type === "error" ? 4500 : 3200,
    });
  }, []);

  const value = React.useMemo(() => ({ show }), [show]);

  return (
    <ToastContext.Provider value={value}>
      <CossToastProvider position="bottom-right">{children}</CossToastProvider>
    </ToastContext.Provider>
  );
}

export function useToast() {
  const context = React.useContext(ToastContext);
  if (!context) throw new Error("useToast must be used inside ToastProvider");
  return context;
}
