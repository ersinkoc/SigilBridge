import { LogIn } from "lucide-react";
import { FormEvent, useState } from "react";

import { api } from "../lib/api";
import { Button } from "../components/ui/Button";
import { Input } from "../components/ui/Input";

export function LoginRoute() {
  const [token, setToken] = useState("");
  const [error, setError] = useState("");

  async function submit(event: FormEvent) {
    event.preventDefault();
    setError("");
    try {
      await api("/admin/v1/auth/login", { method: "POST", body: JSON.stringify({ token }) });
      location.assign(location.pathname.startsWith("/admin/ui") ? "/admin/ui/" : "/");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Login failed");
    }
  }

  return (
    <main className="login">
      <form onSubmit={(event) => void submit(event)} className="login-panel">
        <h1>SigilBridge</h1>
        <label>
          Admin token
          <Input value={token} type="password" onChange={(event) => setToken(event.target.value)} autoFocus />
        </label>
        {error ? <p className="form-error">{error}</p> : null}
        <Button variant="primary" icon={<LogIn size={16} />} type="submit">
          Sign in
        </Button>
      </form>
    </main>
  );
}
