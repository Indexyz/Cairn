import i18n from "i18next";
import { initReactI18next } from "react-i18next";

import en from "@/lib/locales/en";
import ja from "@/lib/locales/ja";
import zh_CN from "@/lib/locales/zh_CN";

export const SUPPORTED_LOCALES = ["en", "zh_CN", "ja"] as const;
export type Locale = (typeof SUPPORTED_LOCALES)[number];

export const LOCALE_STORAGE_KEY = "cairn.locale";

const LOCALE_ALIASES: Record<string, Locale> = {
  en: "en",
  "en-us": "en",
  "en-gb": "en",
  zh: "zh_CN",
  "zh-cn": "zh_CN",
  "zh-hans": "zh_CN",
  "zh-hans-cn": "zh_CN",
  "zh-sg": "zh_CN",
  "zh-tw": "zh_CN",
  "zh-hant": "zh_CN",
  ja: "ja",
  "ja-jp": "ja",
};

export function normalizeLocale(raw: string | null | undefined): Locale {
  if (!raw) return "en";
  const key = raw.trim().toLowerCase().replace(/_/g, "-");
  return LOCALE_ALIASES[key] ?? LOCALE_ALIASES[key.split("-")[0]] ?? "en";
}

function readStoredLocale(): Locale | null {
  try {
    const raw = localStorage.getItem(LOCALE_STORAGE_KEY);
    return raw ? normalizeLocale(raw) : null;
  } catch {
    return null;
  }
}

function detectLocale(): Locale {
  const stored = readStoredLocale();
  if (stored) return stored;
  const navigatorLocale = navigator.languages?.[0] ?? navigator.language;
  return normalizeLocale(navigatorLocale);
}

export function applyDocumentLang(locale: string): void {
  document.documentElement.lang = locale.replace("_", "-");
}

void i18n.use(initReactI18next).init({
  resources: {
    en: { translation: en },
    zh_CN: { translation: zh_CN },
    ja: { translation: ja },
  },
  lng: detectLocale(),
  fallbackLng: "en",
  supportedLngs: [...SUPPORTED_LOCALES],
  load: "currentOnly",
  interpolation: { escapeValue: false },
  returnNull: false,
});

applyDocumentLang(i18n.language);
i18n.on("languageChanged", applyDocumentLang);

export function setLocale(locale: Locale): void {
  try {
    localStorage.setItem(LOCALE_STORAGE_KEY, locale);
  } catch {
    /* storage unavailable */
  }
  void i18n.changeLanguage(locale);
}

export function currentLocale(): Locale {
  return normalizeLocale(i18n.resolvedLanguage ?? i18n.language);
}

export default i18n;
