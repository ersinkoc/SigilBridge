export function WeightSlider({ value, onChange }: { value: number; onChange: (value: number) => void }) {
  return <input aria-label="Weight" type="range" min={1} max={100} value={value} onChange={(event) => onChange(Number(event.target.value))} />;
}
