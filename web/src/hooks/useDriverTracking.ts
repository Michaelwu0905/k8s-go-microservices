'use client';
import { useCallback, useEffect, useRef, useState } from 'react';
import type { Driver, Location } from '@/types';
import { advanceAlongRoute, distanceMeters } from '@/lib/geo';
import { tripRequest } from '@/lib/trips';

type Mode = 'idle' | 'gps' | 'demo';
type Profile = Omit<Driver, 'location'> & { location?: Location };
const startingPoints: Location[] = [
  { latitude: 39.9068, longitude: 116.3746 },
  { latitude: 39.9144, longitude: 116.3860 },
  { latitude: 39.9230, longitude: 116.4100 },
  { latitude: 39.9087, longitude: 116.4301 },
];

export function useDriverTracking(driverId: string) {
  const [mode, setMode] = useState<Mode>('idle');
  const [driver, setDriver] = useState<Driver | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const currentMode = useRef(mode);
  currentMode.current = mode;
  const location = useRef<Location | null>(null);
  const path = useRef<Location[]>([]);
  const availabilityQueue = useRef<Promise<unknown>>(Promise.resolve());
  const setPath = useCallback((coordinates: Location[]) => { path.current = coordinates; }, []);
  const availability = useCallback((id: string, online: boolean) => {
    const next = availabilityQueue.current.catch(() => {}).then(() => tripRequest<Profile>(`/drivers/${id}/availability`, 'PUT', { online }));
    availabilityQueue.current = next;
    return next;
  }, []);

  useEffect(() => {
    setMode('idle'); setDriver(null); setError(null); location.current = null; path.current = [];
    if (!driverId) return;
    const controller = new AbortController();
    void tripRequest<Profile>(`/drivers/${driverId}`, 'GET', undefined, controller.signal).then(profile => {
      if (controller.signal.aborted || currentMode.current !== 'idle') return;
      if (profile.location) {
        location.current = profile.location;
        setDriver({ ...profile, location: profile.location, online: false });
      }
    }).catch(error => { if (!controller.signal.aborted) setError(error.message); });
    return () => controller.abort();
  }, [driverId]);

  useEffect(() => {
    if (!driverId || mode === 'idle') return;
    let active = true;
    let watch: number | undefined;
    let timer: ReturnType<typeof setTimeout>;
    const controller = new AbortController();
    let fix: { location: Location; accuracy: number; timestamp: number } | null = null;
    let posting = false;
    let enabled = false;
    let gpsError = false;
    setError(null); setLoading(true);
    const report = async (point: Location, accuracy = 0) => {
      if (!active || posting) return;
      posting = true;
      try {
        const updated = await tripRequest<Driver>(`/drivers/${driverId}/location`, 'PUT', { location: point, source: mode, accuracy }, controller.signal);
        if (active) { location.current = updated.location; setDriver(updated); setError(null); setLoading(false); }
      } catch (error) {
        if (active) { setError(error instanceof Error ? error.message : '位置上报失败'); setLoading(false); setDriver(previous => previous ? { ...previous, online: false } : null); }
      } finally { posting = false; }
    };
    const tick = async () => {
      if (!active) return;
      if (mode === 'demo') {
        const point = location.current ?? startingPoints[Array.from(driverId).reduce((sum, char) => sum + char.charCodeAt(0), 0) % startingPoints.length] ?? startingPoints[0];
        await report(path.current.length > 1 ? advanceAlongRoute(point, path.current, 25) : point);
      } else if (fix && Date.now() - fix.timestamp < 20000) {
        await report(fix.location, fix.accuracy);
      } else if (enabled && !gpsError) {
        setError('等待有效的设备定位，位置过期后将停止展示');
      }
      if (active) timer = setTimeout(tick, mode === 'demo' ? 1000 : 5000);
    };
    void availability(driverId, true).then(async () => {
      if (!active) return;
      enabled = true;
      if (mode === 'gps') {
        if (!navigator.geolocation) throw new Error('浏览器不支持定位，请使用演示行驶');
        watch = navigator.geolocation.watchPosition(position => {
          gpsError = false;
          fix = { location: { latitude: position.coords.latitude, longitude: position.coords.longitude }, accuracy: position.coords.accuracy, timestamp: position.timestamp };
          void report(fix.location, fix.accuracy);
        }, error => {
          gpsError = true;
          if (active) { fix = null; setError(error.code === 1 ? '定位权限被拒绝，请允许定位或使用演示行驶' : '定位暂不可用，请重试或使用演示行驶'); setLoading(false); }
        }, { enableHighAccuracy: true, maximumAge: 5000, timeout: 15000 });
      }
      await tick();
    }).catch(error => { if (active) { setError(error.message); setLoading(false); } });
    return () => {
      active = false; controller.abort(); clearTimeout(timer);
      if (watch !== undefined) navigator.geolocation.clearWatch(watch);
      void availability(driverId, false).catch(() => {});
    };
  }, [driverId, mode, availability]);

  const stop = useCallback(() => {
    setMode('idle'); setLoading(false); setError(null);
    setDriver(previous => previous ? { ...previous, online: false } : null);
  }, []);
  const startMode = useCallback((next: Mode) => {
    const start = startingPoints[Array.from(driverId).reduce((sum, char) => sum + char.charCodeAt(0), 0) % startingPoints.length] ?? startingPoints[0];
    if (next === 'demo' && location.current && distanceMeters(location.current, start) > 30000) {
      location.current = start;
      path.current = [];
      setDriver(previous => previous ? { ...previous, location: start, online: false, source: 'demo' } : null);
    }
    setMode(next);
  }, [driverId]);
  return { driver, mode, error, loading, setMode: startMode, stop, setPath };
}
