export function Shell({ children, wide }: { children: React.ReactNode; wide?: boolean }) {
  return (
    <main className="shell">
      <div className={wide ? 'stack wide' : 'stack'}>
        <div className="mark" aria-hidden>I</div>
        {children}
      </div>
    </main>
  );
}
