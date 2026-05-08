import { LogOut, Menu, RefreshCw } from "lucide-react";

import { api } from "../../lib/api";
import { LanguageSwitcher } from "./LanguageSwitcher";
import { ThemeSwitcher } from "./ThemeSwitcher";
import { Button } from "../ui/Button";

export function Header({ onMenu }: { onMenu: () => void }) {
  async function logout() {
    await api("/admin/v1/auth/logout", { method: "POST" }).catch(() => undefined);
    location.assign(location.pathname.startsWith("/admin/ui") ? "/admin/ui/login" : "/login");
  }

  return (
    <header className="header">
      <div className="header-main">
        <Button aria-label="Open navigation" className="mobile-only" icon={<Menu size={18} />} onClick={onMenu} />
        <div>
          <div className="header-kicker">
            <span className="status-dot" />
            <span>Admin console</span>
          </div>
          <h1>SigilBridge</h1>
          <p>Provider routing, credentials, audit, and usage controls in one local control plane.</p>
        </div>
      </div>
      <div className="header-actions">
        <Button title="Reload" aria-label="Reload" icon={<RefreshCw size={16} />} onClick={() => window.location.reload()} />
        <Button title="Sign out" aria-label="Sign out" icon={<LogOut size={16} />} onClick={() => void logout()} />
        <LanguageSwitcher />
        <ThemeSwitcher />
      </div>
    </header>
  );
}
