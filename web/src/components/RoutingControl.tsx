import { Polyline } from 'react-leaflet';
import type { Location } from '@/types';

export function RoutingControl({ route, navigation = [], phase }: { route: [number, number][]; navigation?: Location[]; phase?: string }) {
  return <>
    {route.length > 1 && <Polyline positions={route} pathOptions={{ color: navigation.length > 1 ? '#94a3b8' : '#2563eb', weight: 3, opacity: 0.65 }} />}
    {navigation.length > 1 && <Polyline positions={navigation.map(point => [point.latitude, point.longitude])} pathOptions={{ color: phase === 'pickup' ? '#ea580c' : '#2563eb', weight: 5 }} />}
  </>;
}
