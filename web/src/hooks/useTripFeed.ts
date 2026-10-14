'use client';

import { useCallback, useEffect, useState } from 'react';
import { API_URL } from '@/constants';
import { Trip } from '@/types';

// EventSource reconnects automatically. Each connection starts with a DB snapshot.
export function useTripFeed(filter: { riderId?: string; driverId?: string }) {
  const { riderId, driverId } = filter;
  const [trips, setTrips] = useState<Trip[]>([]);
  const [connected, setConnected] = useState(false);
  const [ready, setReady] = useState(false);
  useEffect(() => {
    setTrips([]);
    setReady(false);
    setConnected(false);
    if (!riderId && !driverId) return;
    const params = new URLSearchParams(riderId ? { rider_id: riderId } : { driver_id: driverId! });
    const events = new EventSource(`${API_URL}/events/trips?${params}`);
    events.onmessage = (event) => {
      try {
        const snapshot = JSON.parse(event.data) as { trips: Trip[] };
        setTrips(previous => (snapshot.trips ?? []).map(trip => {
          const existing = previous.find(item => item.trip_id === trip.trip_id);
          return existing && existing.version > trip.version ? existing : trip;
        }));
        setConnected(true);
        setReady(true);
      } catch {
        setConnected(false);
      }
    };
    events.onerror = () => setConnected(false);
    return () => events.close();
  }, [riderId, driverId]);

  const updateTrip = useCallback((trip: Trip) => {
    setTrips(previous => [trip, ...previous.filter(item => item.trip_id !== trip.trip_id)]);
  }, []);
  return { trips, connected, ready, updateTrip };
}

export function useRiderIdentity() {
  const [riderId, setRiderId] = useState('');
  useEffect(() => {
    let id = localStorage.getItem('ride-sharing-rider');
    if (!id || !/^[a-zA-Z0-9_-]{1,64}$/.test(id)) {
      id = `rider-${crypto.randomUUID()}`;
      localStorage.setItem('ride-sharing-rider', id);
    }
    setRiderId(id);
  }, []);
  return riderId;
}
