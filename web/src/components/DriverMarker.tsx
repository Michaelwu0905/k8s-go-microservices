'use client';
import { useEffect, useRef } from 'react';
import L from 'leaflet';
import { Marker, Popup } from 'react-leaflet';
import type { Driver } from '@/types';

const icon = L.divIcon({
  className: '',
  html: '<div style="width:36px;height:36px;overflow:hidden;border-radius:50%;background:white;border:2px solid #171717;box-shadow:0 2px 8px #0003;display:grid;place-items:center"><img src="/car.svg" style="display:block;width:25px;height:25px;object-fit:contain" width="25" height="25" alt="车辆" /></div>',
  iconSize: [36, 36], iconAnchor: [18, 18],
});

export function DriverMarker({ driver }: { driver: Driver }) {
  const marker = useRef<L.Marker>(null);
  const initial = useRef<[number, number]>([driver.location.latitude, driver.location.longitude]);
  const { latitude, longitude } = driver.location;
  useEffect(() => {
    if (!marker.current) return;
    const start = marker.current.getLatLng();
    const started = performance.now();
    let frame: number;
    const animate = (now: number) => {
      const fraction = Math.min(1, (now - started) / 850);
      marker.current?.setLatLng([start.lat + (latitude - start.lat) * fraction, start.lng + (longitude - start.lng) * fraction]);
      if (fraction < 1) frame = requestAnimationFrame(animate);
    };
    frame = requestAnimationFrame(animate);
    return () => cancelAnimationFrame(frame);
  }, [latitude, longitude]);
  return <Marker ref={marker} position={initial.current} icon={icon} opacity={driver.online ? 1 : 0.45}>
    <Popup>司机 {driver.driver_id}<br />{driver.source === 'gps' ? '设备定位' : '演示行驶'} · {driver.online ? '位置实时更新' : '位置已过期或离线'}</Popup>
  </Marker>;
}
