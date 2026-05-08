import i18next from "i18next";
import { initReactI18next } from "react-i18next";

import commonEn from "../locales/en/common.json";
import commonTr from "../locales/tr/common.json";

export const i18n = i18next.createInstance();

void i18n.use(initReactI18next).init({
  lng: localStorage.getItem("sigilbridge-language") ?? "en",
  fallbackLng: "en",
  interpolation: { escapeValue: false },
  resources: {
    en: { common: commonEn },
    tr: { common: commonTr }
  },
  defaultNS: "common"
});

export function setLanguage(language: "en" | "tr") {
  localStorage.setItem("sigilbridge-language", language);
  return i18n.changeLanguage(language);
}
