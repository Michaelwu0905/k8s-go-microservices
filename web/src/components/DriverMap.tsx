'use client';
import 'leaflet/dist/leaflet.css';
import { CircleMarker, MapContainer, Polyline, Popup, TileLayer } from 'react-leaflet';
import { DriverMarker } from '@/components/DriverMarker';
import { MapViewport } from '@/components/MapViewport';
import type { Driver, Location, Trip } from '@/types';

export function DriverMap({ driver, trip, path, phase }: { driver: Driver | null; trip?: Trip; path: Location[]; phase?: string }) {
  const center: [number, number] = driver ? [driver.location.latitude, driver.location.longitude] : [39.9087, 116.3975];
  return <div className="h-[300px] overflow-hidden rounded-xl border sm:h-[400px]">
    <MapContainer center={center} zoom={14} style={{ width: '100%', height: '100%' }}>
      <TileLayer url={`https://t0.tianditu.gov.cn/vec_w/wmts?SERVICE=WMTS&REQUEST=GetTile&VERSION=1.0.0&LAYER=vec&STYLE=default&TILEMATRIXSET=w&FORMAT=tiles&TILEMATRIX={z}&TILEROW={y}&TILECOL={x}&tk=${process.env.NEXT_PUBLIC_TIANDITU_KEY}`} attribution="&copy; 天地图" />
      <TileLayer url={`https://t0.tianditu.gov.cn/cva_w/wmts?SERVICE=WMTS&REQUEST=GetTile&VERSION=1.0.0&LAYER=cva&STYLE=default&TILEMATRIXSET=w&FORMAT=tiles&TILEMATRIX={z}&TILEROW={y}&TILECOL={x}&tk=${process.env.NEXT_PUBLIC_TIANDITU_KEY}`} />
      {driver && <DriverMarker driver={driver} />}
      {trip && <>
        <CircleMarker center={[trip.pickup.latitude, trip.pickup.longitude]} radius={7} pathOptions={{ color: '#16a34a', fillOpacity: 1 }}><Popup>乘客上车点</Popup></CircleMarker>
        <CircleMarker center={[trip.destination.latitude, trip.destination.longitude]} radius={7} pathOptions={{ color: '#2563eb', fillOpacity: 1 }}><Popup>目的地</Popup></CircleMarker>
      </>}
      {path.length > 1 && <Polyline positions={path.map(point => [point.latitude, point.longitude])} pathOptions={{ color: phase === 'pickup' ? '#ea580c' : '#2563eb', weight: 5 }} />}
      <MapViewport path={path} focus={driver?.location} fitKey={`${trip?.trip_id ?? 'idle'}:${phase}:${driver?.source}`} />
    </MapContainer>
  </div>;
}
