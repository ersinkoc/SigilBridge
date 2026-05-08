import { Link } from "react-router-dom";

import { Button } from "../ui/Button";

export function CredentialTabs() {
  return (
    <div className="tabs">
      <Link to="/credentials/api-key/new">
        <Button>API keys</Button>
      </Link>
      <Link to="/credentials/cli">
        <Button>CLI agents</Button>
      </Link>
      <Link to="/credentials/oauth/new">
        <Button>Advanced OAuth</Button>
      </Link>
      <Link to="/credentials/sessions/new">
        <Button>Session fallback</Button>
      </Link>
    </div>
  );
}
