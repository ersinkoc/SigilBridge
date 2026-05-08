import { Button } from "../ui/Button";

type Props = {
  open: boolean;
  title: string;
  onCancel: () => void;
  onConfirm: () => void;
};

export function ConfirmDialog({ open, title, onCancel, onConfirm }: Props) {
  if (!open) {
    return null;
  }
  return (
    <div className="modal-backdrop" role="presentation">
      <div className="modal" role="dialog" aria-modal="true" aria-label={title}>
        <h2>{title}</h2>
        <div className="modal-actions">
          <Button onClick={onCancel}>Cancel</Button>
          <Button variant="primary" onClick={onConfirm}>
            Confirm
          </Button>
        </div>
      </div>
    </div>
  );
}
