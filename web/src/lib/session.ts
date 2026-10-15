import { API_URL } from '@/constants';
let token = '';
let checking: Promise<void> | null = null;
export function setCSRFToken(value: string) { token = value; }
export function getCSRFToken() { return token; }
export function sessionExpired() {
  if (typeof window !== 'undefined') window.dispatchEvent(new Event('ride-auth-expired'));
}
export function checkSession() {
  if (checking) return checking;
  checking = fetch(`${API_URL}/auth/me`, { credentials: 'include', cache: 'no-store', signal: AbortSignal.timeout(10000) })
    .then(response => { if (response.status === 401) sessionExpired(); })
    .catch(() => {})
    .finally(() => { checking = null; });
  return checking;
}
