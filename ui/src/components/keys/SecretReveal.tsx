import { Copy } from "lucide-react";

import { Button } from "../ui/Button";
import { Card } from "../ui/Card";

export function SecretReveal({ plaintext }: { plaintext: string }) {
  if (!plaintext) {
    return null;
  }
  return (
    <Card className="secret-reveal">
      <strong>One-time secret</strong>
      <code>{plaintext}</code>
      <Button icon={<Copy size={16} />} onClick={() => void navigator.clipboard.writeText(plaintext)}>
        Copy
      </Button>
    </Card>
  );
}
