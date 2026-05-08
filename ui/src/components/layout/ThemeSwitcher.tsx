import { Monitor, Moon, Sun } from "lucide-react";
import type { ReactNode } from "react";

import { useTheme, type ThemeMode } from "../../lib/theme";
import { Button } from "../ui/Button";

const modes: Array<{ mode: ThemeMode; icon: ReactNode; label: string }> = [
  { mode: "light", icon: <Sun size={16} />, label: "Light" },
  { mode: "dark", icon: <Moon size={16} />, label: "Dark" },
  { mode: "system", icon: <Monitor size={16} />, label: "System" }
];

export function ThemeSwitcher() {
  const { mode, setMode } = useTheme();
  return (
    <div className="segmented" aria-label="Theme">
      {modes.map((item) => (
        <Button
          key={item.mode}
          title={item.label}
          aria-label={item.label}
          icon={item.icon}
          variant={mode === item.mode ? "primary" : "ghost"}
          onClick={() => setMode(item.mode)}
        />
      ))}
    </div>
  );
}
