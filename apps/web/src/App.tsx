import { useEffect, useState } from 'react';

export function App() {
  const [status, setStatus] = useState('checking…');

  useEffect(() => {
    fetch('/api/healthz')
      .then((r) => r.json())
      .then((d: { status: string }) => setStatus(d.status))
      .catch(() => setStatus('unreachable'));
  }, []);

  return (
    <main style={{ fontFamily: 'system-ui', padding: '2rem' }}>
      <h1>Impressipies</h1>
      <p>API status: {status}</p>
    </main>
  );
}
