import { create } from "zustand";

export type ThemeMode = "light" | "dark" | "system";

type ThemeState = {
  mode: ThemeMode;
  setMode: (mode: ThemeMode) => void;
};

const key = "sigilbridge-theme";

function apply(mode: ThemeMode) {
  const dark = mode === "dark" || (mode === "system" && window.matchMedia("(prefers-color-scheme: dark)").matches);
  document.documentElement.dataset.theme = dark ? "dark" : "light";
  localStorage.setItem(key, mode);
}

export const useTheme = create<ThemeState>((set) => ({
  mode: (localStorage.getItem(key) as ThemeMode | null) ?? "system",
  setMode: (mode) => {
    apply(mode);
    set({ mode });
  }
}));

export function initTheme() {
  apply((localStorage.getItem(key) as ThemeMode | null) ?? "system");
}
