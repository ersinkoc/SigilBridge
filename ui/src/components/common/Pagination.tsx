import { ChevronLeft, ChevronRight } from "lucide-react";

import { Button } from "../ui/Button";

export function Pagination({ onPrev, onNext, disabled }: { onPrev?: () => void; onNext?: () => void; disabled?: boolean }) {
  return (
    <div className="pagination">
      <Button aria-label="Previous page" icon={<ChevronLeft size={16} />} onClick={onPrev} disabled={disabled} />
      <Button aria-label="Next page" icon={<ChevronRight size={16} />} onClick={onNext} disabled={disabled} />
    </div>
  );
}
