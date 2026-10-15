'use client';
import { useState } from 'react';
import { useAuth } from '@/components/AuthProvider';
import { Button } from '@/components/ui/button';

export function AccountMenu() {
  const { user, logout } = useAuth();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  return <div className="flex flex-wrap items-center gap-2 text-sm">
    <span>{user?.display_name} · {user?.role === 'driver' ? '司机' : '乘客'}</span>
    <Button variant="outline" size="sm" disabled={busy} onClick={async () => {
      setBusy(true); setError(null);
      try { await logout(); } catch (error) { setError(error instanceof Error ? error.message : '退出失败'); }
      finally { setBusy(false); }
    }}>{busy ? '退出中…' : '退出登录'}</Button>
    {error && <span role="alert" className="text-xs text-destructive">{error}</span>}
  </div>;
}
