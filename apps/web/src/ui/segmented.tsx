import { cn } from './cn';

/** A two-or-more-way toggle drawn as one pill, e.g. Password | Email link. */
export function Segmented<T extends string>({
  options, value, onChange, label,
}: {
  options: { value: T; label: React.ReactNode }[];
  value: T;
  onChange: (v: T) => void;
  label: string;
}) {
  return (
    <div
      className="grid gap-1 rounded-md border-2 border-outline bg-surface-2 p-1"
      style={{ gridTemplateColumns: `repeat(${options.length}, minmax(0, 1fr))` }}
      role="group"
      aria-label={label}
    >
      {options.map((o) => (
        <button
          key={o.value}
          type="button"
          aria-pressed={value === o.value}
          onClick={() => value !== o.value && onChange(o.value)}
          className={cn(
            'inline-flex min-h-10 items-center justify-center gap-2 rounded-sm border-0 px-2 text-sm font-bold [&_svg]:size-4',
            value === o.value ? 'bg-surface text-ink shadow-[0_0_0_2px_var(--color-outline)]' : 'bg-transparent text-ink-muted hover:text-ink',
          )}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}
