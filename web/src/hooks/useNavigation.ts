'use client';
import { useEffect, useRef, useState } from 'react';
import type { Driver, Location, Navigation, Trip } from '@/types';
import { TripStatus } from '@/types';
import { distanceMeters, remainingRoute, routeLength } from '@/lib/geo';
import { tripRequest } from '@/lib/trips';

export function useNavigation(trip: Trip | null | undefined, driver?: Driver | null) {
  const [navigation, setNavigation] = useState<Navigation | null>(null);
  const [error, setError] = useState<string | null>(null);
  const position = useRef(driver?.location);
  position.current = driver?.location;
  const tripId = trip?.trip_id;
  const phase = trip?.status === TripStatus.InTrip ? 'destination' : 'pickup';
  const eligible = !!trip?.driver_id && trip.status >= TripStatus.Accepted && trip.status <= TripStatus.InTrip;
  const online = !!driver?.online;
  useEffect(() => {
    setNavigation(null);
    setError(null);
    if (!tripId || !eligible || !online) return;
    const controller = new AbortController();
    let origin: Location | undefined;
    let loading = false;
    const load = async () => {
      if (loading || !position.current || origin && distanceMeters(origin, position.current) < 80) return;
      loading = true;
      try {
        const data = await tripRequest<Navigation>(`/trip/${tripId}/navigation`, 'GET', undefined, controller.signal);
        if (!controller.signal.aborted) {
          origin = data.driver.location;
          setNavigation(data);
          setError(null);
        }
      } catch (error) {
        if (!controller.signal.aborted) setError(error instanceof Error ? error.message : '导航路线暂不可用');
      } finally { loading = false; }
    };
    void load();
    const interval = setInterval(load, 10000);
    return () => { controller.abort(); clearInterval(interval); };
  }, [tripId, phase, eligible, online]);

  const current = navigation && navigation.trip_id === tripId && navigation.phase === phase && eligible ? navigation : null;
  const path = current?.route.geometry[0]?.coordinates ?? [];
  const remaining = driver?.location ? remainingRoute(driver.location, path) : path;
  const total = routeLength(path);
  const ratio = total > 0 ? Math.min(1, routeLength(remaining) / total) : 0;
  return {
    navigation: current,
    path: remaining,
    distance: current ? current.route.distance * ratio : null,
    duration: current ? current.route.duration * ratio : null,
    error,
  };
}
