'use client';
import { createContext, useCallback, useContext, useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { API_URL } from '@/constants';
import { setCSRFToken } from '@/lib/session';
import { tripRequest } from '@/lib/trips';

export interface User {
  id: string;
  username: string;
  display_name: string;
  role: 'rider' | 'driver';
  csrf_token: string;
  expires_at: string;
}
interface AuthState {
  user: User | null;
  ready: boolean;
  signIn: (path: '/auth/login' | '/auth/register', body: unknown) => Promise<User>;
  logout: () => Promise<void>;
}
const AuthContext = createContext<AuthState | null>(null);
const changeKey = 'ride-sharing-auth-changed';

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [ready, setReady] = useState(false);
  const adopt = useCallback((user: User | null) => { setCSRFToken(user?.csrf_token ?? ''); setUser(user); setReady(true); }, []);
  const refresh = useCallback(async () => {
    try {
      const response = await fetch(`${API_URL}/auth/me`, { credentials: 'include', cache: 'no-store', signal: AbortSignal.timeout(10000) });
      if (response.ok) adopt(await response.json());
      else if (response.status === 401) adopt(null);
      else setReady(true);
    } catch { setReady(true); }
  }, [adopt]);
  useEffect(() => {
    void refresh();
    const expired = () => adopt(null);
    const changed = (event: StorageEvent) => { if (event.key === changeKey) void refresh(); };
    window.addEventListener('ride-auth-expired', expired);
    window.addEventListener('storage', changed);
    return () => { window.removeEventListener('ride-auth-expired', expired); window.removeEventListener('storage', changed); };
  }, [refresh, adopt]);
  const signIn = async (path: '/auth/login' | '/auth/register', body: unknown) => {
    const signedIn = await tripRequest<User>(path, 'POST', body);
    adopt(signedIn);
    localStorage.setItem(changeKey, `${Date.now()}-${Math.random()}`);
    return signedIn;
  };
  const logout = async () => {
    await tripRequest('/auth/logout', 'POST');
    adopt(null);
    localStorage.setItem(changeKey, `${Date.now()}-${Math.random()}`);
  };
  return <AuthContext.Provider value={{ user, ready, signIn, logout }}>{children}</AuthContext.Provider>;
}
export function useAuth() {
  const state = useContext(AuthContext);
  if (!state) throw new Error('AuthProvider is required');
  return state;
}
export function RequireAuth({ role, children }: { role: User['role']; children: React.ReactNode }) {
  const { user, ready } = useAuth();
  const router = useRouter();
  useEffect(() => {
    if (!ready) return;
    if (!user) router.replace('/login');
    else if (user.role !== role) router.replace(user.role === 'driver' ? '/driver' : '/');
  }, [user, ready, role, router]);
  if (!ready || !user || user.role !== role) return <main className="grid min-h-screen place-items-center text-sm text-muted-foreground">正在确认登录状态…</main>;
  return <>{children}</>;
}
