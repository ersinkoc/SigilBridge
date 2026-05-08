import { Button } from "../ui/Button";

type Props = {
  open: boolean;
  title: string;
  description?: string;
  confirmLabel?: string;
  cancelLabel?: string;
  busy?: boolean;
  onCancel: () => void;
  onConfirm: () => void;
};

export function ConfirmDialog({ open, title, description, confirmLabel = "Confirm", cancelLabel = "Cancel", busy = false, onCancel, onConfirm }: Props) {
  if (!open) {
    return null;
  }
  return (
    <div className="modal-backdrop" role="presentation" onMouseDown={onCancel}>
      <div className="modal confirm-modal" role="dialog" aria-modal="true" aria-labelledby="confirm-dialog-title" aria-describedby={description ? "confirm-dialog-description" : undefined} onMouseDown={(event) => event.stopPropagation()}>
        <div>
          <h2 id="confirm-dialog-title">{title}</h2>
          {description ? <p id="confirm-dialog-description">{description}</p> : null}
        </div>
        <div className="modal-actions">
          <Button onClick={onCancel} disabled={busy}>
            {cancelLabel}
          </Button>
          <Button variant="danger" onClick={onConfirm} disabled={busy}>
            {busy ? "Working" : confirmLabel}
          </Button>
        </div>
      </div>
    </div>
  );
}
