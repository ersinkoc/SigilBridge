import { Languages } from "lucide-react";
import { useTranslation } from "react-i18next";

import { setLanguage } from "../../lib/i18n";
import { Button } from "../ui/Button";

export function LanguageSwitcher() {
  const { i18n } = useTranslation();
  const current = i18n.language === "tr" ? "tr" : "en";

  return (
    <div className="segmented" aria-label="Language">
      <Button
        title="English"
        icon={<Languages size={16} />}
        variant={current === "en" ? "primary" : "ghost"}
        onClick={() => void setLanguage("en")}
      >
        EN
      </Button>
      <Button title="Turkish" variant={current === "tr" ? "primary" : "ghost"} onClick={() => void setLanguage("tr")}>
        TR
      </Button>
    </div>
  );
}
