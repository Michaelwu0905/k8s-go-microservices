'use client';

import 'leaflet/dist/leaflet.css';
import { useNearbyDrivers } from '@/hooks/useNearbyDrivers';
import { MapContainer, Marker, Popup, Rectangle, TileLayer } from 'react-leaflet'
import L from 'leaflet';
import { decodeGeoHash } from '@/utils/geohash';
import { useEffect, useState } from 'react';
import Link from 'next/link';
import { useRiderIdentity, useTripFeed } from '@/hooks/useTripFeed';
import { disableMapPropagation } from '@/lib/mapInteractions';
import { tripRequest } from '@/lib/trips';
// Fix for default marker icon
import icon from 'leaflet/dist/images/marker-icon.png'
import iconShadow from 'leaflet/dist/images/marker-shadow.png'
import { MapClickHandler } from './MapClickHandler';
import { isActiveTrip, Trip, TripStatus } from "@/types";
import { RoutingControl } from "@/components/RoutingControl";
import { RideConfirmationCard } from "@/components/RideConfirmationCard";

const DefaultIcon = L.icon({
    iconUrl: icon.src,
    shadowUrl: iconShadow.src,
    iconSize: [25, 41],
    iconAnchor: [12, 41],
})

L.Marker.prototype.options.icon = DefaultIcon


export function NearbyDriversMap() {
    const riderId = useRiderIdentity();
    const { trips, connected, ready, updateTrip } = useTripFeed({ riderId });
    const [trackedId, setTrackedId] = useState('');
    const [destination, setDestination] = useState<[number, number] | null>(null);
    const [busy, setBusy] = useState(false);
    const [requestError, setRequestError] = useState<string | null>(null);
    const activeTrip = trips.find(isActiveTrip);
    const trip = trips.find(item => item.trip_id === trackedId) ?? activeTrip ?? null;
    const route = trip?.route.geometry[0]?.coordinates.map(point => [point.latitude, point.longitude] as [number, number]) ?? [];
    const location = { latitude: 39.9087, longitude: 116.3975 };
    const { drivers, error } = useNearbyDrivers(location);

    useEffect(() => {
        if (activeTrip) setTrackedId(activeTrip.trip_id);
    }, [activeTrip]);

    const handleMapClick = (e: L.LeafletMouseEvent) => {
        if (trip || busy) return;
        setDestination([e.latlng.lat, e.latlng.lng]);
        setRequestError(null);
    };

    const requestRide = async () => {
        if (!destination || !riderId || busy) return;
        setBusy(true);
        setRequestError(null);
        try {
            const created = await tripRequest<Trip>('/trip', 'POST', {
                rider_id: riderId, pickup: location,
                destination: { latitude: destination[0], longitude: destination[1] },
            });
            setTrackedId(created.trip_id);
            updateTrip(created);
        } catch (error) {
            setRequestError(error instanceof Error ? error.message : '创建订单失败');
        } finally { setBusy(false); }
    };

    const cancelRide = async () => {
        if (!trip || busy) return;
        setBusy(true);
        setRequestError(null);
        try {
            updateTrip(await tripRequest<Trip>(`/trip/${trip.trip_id}/status`, 'PATCH', {
                rider_id: riderId, status: TripStatus.Cancelled,
            }));
        } catch (error) {
            setRequestError(error instanceof Error ? error.message : '取消订单失败');
        } finally { setBusy(false); }
    };

    const userMarker = new L.Icon({
        iconUrl: "https://upload.wikimedia.org/wikipedia/commons/thumb/e/ed/Map_pin_icon.svg/176px-Map_pin_icon.svg.png",
        iconSize: [40, 40], // Size of the marker
        iconAnchor: [20, 40], // Anchor point
    });

    const driverMarker = new L.Icon({
        iconUrl: "https://www.svgrepo.com/show/25407/car.svg",
        iconSize: [30, 30],
        iconAnchor: [15, 30],
    });

    // Function to create grid bounds from geohash
    const getGeohashBounds = (geohash: string) => {
        const { latitude: [minLat, maxLat], longitude: [minLng, maxLng] } = decodeGeoHash(geohash);
        return [
            [minLat, minLng],
            [maxLat, maxLng],
        ];
    };

    return (
        <MapContainer
            center={[location.latitude, location.longitude]}
            zoom={13}
            style={{ height: '100vh', width: '100%' }}
        >
            <TileLayer
                url={`https://t0.tianditu.gov.cn/vec_w/wmts?SERVICE=WMTS&REQUEST=GetTile&VERSION=1.0.0&LAYER=vec&STYLE=default&TILEMATRIXSET=w&FORMAT=tiles&TILEMATRIX={z}&TILEROW={y}&TILECOL={x}&tk=${process.env.NEXT_PUBLIC_TIANDITU_KEY}`}
                attribution="&copy; 天地图"
            />
            <TileLayer
                url={`https://t0.tianditu.gov.cn/cva_w/wmts?SERVICE=WMTS&REQUEST=GetTile&VERSION=1.0.0&LAYER=cva&STYLE=default&TILEMATRIXSET=w&FORMAT=tiles&TILEMATRIX={z}&TILEROW={y}&TILECOL={x}&tk=${process.env.NEXT_PUBLIC_TIANDITU_KEY}`}
                attribution="&copy; 天地图"
            />
            <Marker position={[location.latitude, location.longitude]} icon={userMarker} />

            {/* Render geohash grid cells */}
            {drivers.map((driver) => (
                <Rectangle
                    key={`grid-${driver.geohash}`}
                    bounds={getGeohashBounds(driver.geohash) as L.LatLngBoundsExpression}
                    pathOptions={{
                        color: '#3388ff',
                        weight: 1,
                        fillOpacity: 0.1
                    }}
                >
                    <Popup>Geohash: {driver.geohash}</Popup>
                </Rectangle>
            ))}

            {/* Render driver markers */}
            {drivers.map((driver) => (
                <Marker
                    key={driver.driver_id}
                    position={[driver.location.latitude, driver.location.longitude]}
                    icon={driverMarker}
                >
                    <Popup>
                        Driver ID: {driver.driver_id}
                        <br />
                        Geohash: {driver.geohash}
                    </Popup>
                </Marker>
            ))}
            {(trip?.destination || destination) && (
                <Marker position={trip ? [trip.destination.latitude, trip.destination.longitude] : destination!} icon={userMarker}>
                    <Popup>目的地</Popup>
                </Marker>
            )}
            <div ref={disableMapPropagation} className="absolute right-4 top-4 z-[1000] max-w-[calc(100%-5rem)] rounded-xl border bg-background p-3 shadow">
                <div className="flex items-center gap-4 text-sm">
                    <span>{connected ? '订单实时同步中' : ready ? '连接中断，正在重连…' : '正在恢复订单…'}</span>
                    <Link href="/driver" className="font-medium underline">司机端</Link>
                </div>
                {error && <p className="mt-1 text-xs text-muted-foreground">司机位置暂不可用，订单仍可操作。</p>}
            </div>
            <RideConfirmationCard
                trip={trip}
                destination={destination ? { latitude: destination[0], longitude: destination[1] } : null}
                busy={busy}
                error={requestError}
                canRequest={ready && connected}
                onRequest={requestRide}
                onCancel={cancelRide}
                onReset={() => { setTrackedId(''); setDestination(null); setRequestError(null); }}
            />
            <RoutingControl
                route={route}
            />
            <MapClickHandler onClick={handleMapClick} />
        </MapContainer>
    )
}