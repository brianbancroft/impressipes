export function Toast({ children }: { children: React.ReactNode }) {
  return (
    <div
      role="status"
      className="fixed bottom-28 left-1/2 z-50 flex max-w-[calc(100vw-2rem)] -translate-x-1/2 items-center gap-3 rounded-full border-[2.5px] border-outline bg-rail px-5 py-3 font-bold text-on-rail shadow-pop-sm md:bottom-8"
    >
      {children}
    </div>
  );
}
