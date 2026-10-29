export function Loading({ label = 'Loading…' }: { label?: string }) {
  return <p className="sub" role="status">{label}</p>;
}

export function ErrorState({ onRetry }: { onRetry?: () => void }) {
  return (
    <div role="alert">
      <h1>Something went wrong</h1>
      <p className="sub">Check your connection and try again.</p>
      {onRetry && <button onClick={onRetry}>Retry</button>}
    </div>
  );
}

export function EmptyState({ emoji, title, body }: { emoji: string; title: string; body: string }) {
  return (
    <div className="empty">
      <div className="text-5xl" aria-hidden>{emoji}</div>
      <h2 className="text-xl">{title}</h2>
      <p className="text-ink-muted">{body}</p>
    </div>
  );
}
