'use client';

import 'leaflet/dist/leaflet.css';
import { useNearbyDrivers } from '@/hooks/useNearbyDrivers';
import { CircleMarker, MapContainer, Popup, TileLayer } from 'react-leaflet'
import type L from 'leaflet';
import { DriverMarker } from '@/components/DriverMarker';
import { MapViewport } from '@/components/MapViewport';
import { useNavigation } from '@/hooks/useNavigation';
import { useEffect, useState } from 'react';
import { useAuth } from '@/components/AuthProvider';
import { AccountMenu } from '@/components/AccountMenu';
import { useTripFeed } from '@/hooks/useTripFeed';
import { disableMapPropagation } from '@/lib/mapInteractions';
import { tripRequest } from '@/lib/trips';
import { MapClickHandler } from './MapClickHandler';
import { isActiveTrip, Trip, TripStatus } from "@/types";
import { RoutingControl } from "@/components/RoutingControl";
import { RideConfirmationCard } from "@/components/RideConfirmationCard";

export function NearbyDriversMap() {
    const { user } = useAuth();
    const riderId = user?.id ?? '';
    const { trips, connected, ready, updateTrip } = useTripFeed({ riderId });
    const [trackedId, setTrackedId] = useState('');
    const [destination, setDestination] = useState<[number, number] | null>(null);
    const [busy, setBusy] = useState(false);
    const [requestError, setRequestError] = useState<string | null>(null);
    const activeTrip = trips.find(isActiveTrip);
    const trip = trips.find(item => item.trip_id === trackedId) ?? activeTrip ?? null;
    const route = trip?.route.geometry[0]?.coordinates.map(point => [point.latitude, point.longitude] as [number, number]) ?? [];
    const location = { latitude: 39.9087, longitude: 116.3975 };
    const { drivers, error } = useNearbyDrivers(location, trip && isActiveTrip(trip) ? trip.driver_id : undefined);
    const assignedDriver = drivers.find(driver => driver.driver_id === trip?.driver_id);
    const navigation = useNavigation(trip, assignedDriver);

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
                pickup: location,
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
                status: TripStatus.Cancelled,
            }));
        } catch (error) {
            setRequestError(error instanceof Error ? error.message : '取消订单失败');
        } finally { setBusy(false); }
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
            <CircleMarker center={[location.latitude, location.longitude]} radius={8} bubblingMouseEvents={false} pathOptions={{ color: '#16a34a', fillOpacity: 1 }}><Popup>上车点</Popup></CircleMarker>

            {drivers.filter(driver => driver.online || driver.driver_id === trip?.driver_id).map(driver => (
                <DriverMarker key={driver.driver_id} driver={driver} />
            ))}
            {(trip?.destination || destination) && (
                <CircleMarker center={trip ? [trip.destination.latitude, trip.destination.longitude] : destination!} radius={8} bubblingMouseEvents={false} pathOptions={{ color: '#2563eb', fillOpacity: 1 }}>
                    <Popup>目的地</Popup>
                </CircleMarker>
            )}
            <div ref={disableMapPropagation} className="absolute right-4 top-4 z-[1000] max-w-[calc(100%-5rem)] rounded-xl border bg-background p-3 shadow">
                <div className="flex items-center gap-4 text-sm">
                    <span>{connected ? '订单实时同步中' : ready ? '连接中断，正在重连…' : '正在恢复订单…'}</span>
                    <AccountMenu />
                </div>
                {error && <p className="mt-1 text-xs text-muted-foreground">司机位置暂不可用，订单仍可操作。</p>}
            </div>
            <RideConfirmationCard
                trip={trip}
                driver={assignedDriver}
                navigation={navigation.navigation}
                remainingDistance={navigation.distance}
                remainingDuration={navigation.duration}
                navigationError={navigation.error}
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
                navigation={navigation.path}
                phase={navigation.navigation?.phase}
            />
            {trip?.driver_id && <MapViewport path={navigation.path} focus={assignedDriver?.location} fitKey={`${trip.trip_id}:${navigation.navigation?.phase}`} />}
            <MapClickHandler onClick={handleMapClick} />
        </MapContainer>
    )
}