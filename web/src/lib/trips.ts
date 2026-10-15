import { API_URL } from '@/constants';
import { Location } from '@/types';

export async function tripRequest<T>(path: string, method = 'GET', body?: unknown, signal?: AbortSignal): Promise<T> {
  const response = await fetch(`${API_URL}${path}`, {
    method,
    headers: body ? { 'Content-Type': 'application/json' } : undefined,
    body: body ? JSON.stringify(body) : undefined,
    signal: signal ? AbortSignal.any([signal, AbortSignal.timeout(20000)]) : AbortSignal.timeout(20000),
  });
  const result = await response.json();
  if (!response.ok) throw new Error(result.error || `请求失败（${response.status}）`);
  return result as T;
}

export function formatLocation(location: Location): string {
  return `${location.latitude.toFixed(4)}, ${location.longitude.toFixed(4)}`;
}

export function formatDistance(meters: number): string {
  return meters < 1000 ? `${Math.round(meters)} 米` : `${(meters / 1000).toFixed(1)} 公里`;
}

export function formatDuration(seconds: number): string {
  return `约 ${Math.max(1, Math.round(seconds / 60))} 分钟`;
}
