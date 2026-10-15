'use client';
import { useEffect, useState } from 'react';
import { WEBSOCKET_URL } from '@/constants';
import { checkSession } from '@/lib/session';
import type { Driver, Location } from '@/types';
export type { Driver } from '@/types';

export function useNearbyDrivers(location: Location, assignedDriverId = '') {
  const [drivers, setDrivers] = useState<Driver[]>([]);
  const [error, setError] = useState<string | null>(null);
  const { latitude, longitude } = location;
  useEffect(() => {
    let active = true;
    let socket: WebSocket;
    let retry: ReturnType<typeof setTimeout>;
    let attempts = 0;
    const connect = () => {
      if (!active) return;
      socket = new WebSocket(`${WEBSOCKET_URL}/drivers`);
      socket.onopen = () => {
        if (!active) return;
        socket.send(JSON.stringify({ latitude, longitude, assigned_driver_id: assignedDriverId }));
      };
      socket.onmessage = event => {
        if (!active) return;
        try {
          setDrivers((JSON.parse(event.data) ?? []) as Driver[]);
          setError(null);
          attempts = 0;
        } catch { setError('司机位置数据暂不可用'); }
      };
      socket.onerror = () => { if (active) setError('位置连接中断，正在重连'); };
      socket.onclose = () => {
        if (!active) return;
        setError('位置连接中断，正在重连');
        void checkSession();
        setDrivers(previous => previous.map(driver => ({ ...driver, online: false })));
        retry = setTimeout(connect, Math.min(10000, 1000 * 2 ** attempts++));
      };
    };
    connect();
    return () => { active = false; clearTimeout(retry); socket?.close(); };
  }, [latitude, longitude, assignedDriverId]);
  return { drivers, error };
}
