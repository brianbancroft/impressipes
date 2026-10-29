import { useEffect, useRef, useState } from 'react';
import { Link } from '@tanstack/react-router';
import { verifyLink } from './api';
import { Shell } from './Shell';
import { Loading } from './states';

/** Landing page for the emailed link: redeems the token, then goes home. */
export function VerifyLink({ token }: { token: string }) {
  const [error, setError] = useState('');
  // The token works once, so React's double-invoked effect must not redeem it twice.
  const started = useRef(false);

  useEffect(() => {
    if (started.current) return;
    started.current = true;
    verifyLink(token).then((res) => {
      if (!res.ok) return setError(res.message);
      window.location.replace(res.member.hasPassword ? '/' : '/account?welcome=1');
    });
  }, [token]);

  if (!error) return <Shell><Loading /></Shell>;
  return (
    <Shell>
      <h1 className="text-4xl">That link didn’t work</h1>
      <p className="mt-2 text-ink-muted" role="alert">{error}. Links work once and expire after 15 minutes.</p>
      <p className="mt-4"><Link to="/signin">Back to sign in</Link></p>
    </Shell>
  );
}
