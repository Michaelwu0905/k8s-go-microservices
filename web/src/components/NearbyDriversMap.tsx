'use client';

import 'leaflet/dist/leaflet.css';
import { Driver, useNearbyDrivers } from '@/hooks/useNearbyDrivers';
import { MapContainer, Marker, Popup, Rectangle, TileLayer } from 'react-leaflet'
import L from 'leaflet';
import { decodeGeoHash } from '@/utils/geohash';
import { useRef, useState } from 'react';
// Fix for default marker icon
import icon from 'leaflet/dist/images/marker-icon.png'
import iconShadow from 'leaflet/dist/images/marker-shadow.png'
import { MapClickHandler } from './MapClickHandler';
import { DriverList } from './DriversList';
import { RequestRideProps, RouteInfo } from "@/types";
import { RoutingControl } from "@/components/RoutingControl";
import { RideConfirmationCard } from "@/components/RideConfirmationCard";
import { API_URL } from '../constants';

const DefaultIcon = L.icon({
    iconUrl: icon.src,
    shadowUrl: iconShadow.src,
    iconSize: [25, 41],
    iconAnchor: [12, 41],
})

L.Marker.prototype.options.icon = DefaultIcon


export function NearbyDriversMap() {
    const [selectedDriver, setSelectedDriver] = useState<Driver | null>(null)
    const [route, setRoute] = useState<[number, number][]>([])
    const [tripInfo, setTripInfo] = useState<{ tripId: string; distance: number; duration: number } | null>(null)
    const mapRef = useRef<L.Map>(null)

    const location = {
        latitude: 39.9087,
        longitude: 116.3975,
    };

    const [destination, setDestination] = useState<[number, number] | null>(null)

    const handleMapClick = async (e: L.LeafletMouseEvent) => {
        setDestination([e.latlng.lat, e.latlng.lng])
        
        await requestRide({
            pickup: [location.latitude, location.longitude],
            destination: [e.latlng.lat, e.latlng.lng],
        })
    }

    const { drivers, error } = useNearbyDrivers(location);

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

    const requestRide = async (props: RequestRideProps): Promise<RouteInfo> => {
        const { pickup, destination } = props
        const payload = {
            pickup: {
                latitude: pickup[0],
                longitude: pickup[1],
            },
            destination: {
                latitude: destination[0],
                longitude: destination[1],
            },
        }

        const response = await fetch(`${API_URL}/trip`, {
            method: 'POST',
            body: JSON.stringify(payload),
        })
        const data = await response.json() as RouteInfo
        const route = data.route
        const parsedRoute = route.geometry[0].coordinates
            .map((coord) => [coord.latitude, coord.longitude] as [number, number])

        setRoute(parsedRoute)
        setTripInfo({ tripId: data.trip_id, distance: route.distance, duration: route.duration })
        return data
    }

    if (error) {
        return <div>Error: {error}</div>;
    }

    return (
        <MapContainer
            center={[location.latitude, location.longitude]}
            zoom={13}
            style={{ height: '100vh', width: '100%' }}
            ref={mapRef}
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
            {destination && (
                <Marker position={destination} icon={userMarker}>
                    <Popup>Destination</Popup>
                </Marker>
            )}
            {destination ? (
                <DriverList drivers={drivers} onSelectDriver={setSelectedDriver} />
            ) : (
                <div className="flex items-center justify-center h-full">
                    <p className="text-lg font-semibold text-gray-500">Click on the map to set a destination</p>
                </div>
            )}
            {selectedDriver && tripInfo && (
                <RideConfirmationCard
                    driver={selectedDriver}
                    tripId={tripInfo.tripId}
                    distance={tripInfo.distance}
                    duration={tripInfo.duration}
                    onCancel={() => setSelectedDriver(null)}
                />
            )}
            <RoutingControl
                route={route}
            />
            <MapClickHandler onClick={handleMapClick} />
        </MapContainer>
    )
}