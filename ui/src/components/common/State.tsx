import { AlertTriangle, Inbox } from "lucide-react";

export function EmptyState({ label = "No records" }: { label?: string }) {
  return (
    <div className="state">
      <Inbox size={18} />
      <span>{label}</span>
    </div>
  );
}

export function ErrorState({ label = "Something went wrong" }: { label?: string }) {
  return (
    <div className="state state-error">
      <AlertTriangle size={18} />
      <span>{label}</span>
    </div>
  );
}

export function Skeleton() {
  return <div className="skeleton" />;
}
