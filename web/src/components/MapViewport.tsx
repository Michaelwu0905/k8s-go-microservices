'use client';
import { useEffect, useRef } from 'react';
import { useMap } from 'react-leaflet';
import type { Location } from '@/types';

export function MapViewport({ path, focus, fitKey }: { path: Location[]; focus?: Location; fitKey: string }) {
  const map = useMap();
  const fitted = useRef('');
  const key = `${fitKey}:${path.length > 1 ? 'route' : focus ? 'position' : 'empty'}`;
  useEffect(() => {
    if (fitted.current === key) return;
    if (path.length > 1) map.fitBounds(path.map(point => [point.latitude, point.longitude]), { padding: [40, 40], maxZoom: 16 });
    else if (focus) map.setView([focus.latitude, focus.longitude], 14);
    else return;
    fitted.current = key;
  }, [map, key, path, focus]);
  return null;
}
