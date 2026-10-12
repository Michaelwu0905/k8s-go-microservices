import { useEffect, useState } from 'react';
import { WEBSOCKET_URL } from "@/constants";

interface Location {
  latitude: number;
  longitude: number;
}

export interface Driver {
  driver_id: string;
  location: Location;
  geohash: string;
}

export function useNearbyDrivers(location: Location) {
  const [drivers, setDrivers] = useState<Driver[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const ws = new WebSocket(`${WEBSOCKET_URL}/drivers`);
    let active = true;

    ws.onopen = () => {
      if (!active) return;
      setError(null);
      // Send initial location
      ws.send(JSON.stringify(location));
    };

    ws.onmessage = (event) => {
      if (!active) return;
      const drivers = JSON.parse(event.data) as Driver[];
      setDrivers(drivers);
    };

    ws.onclose = () => {
      console.log('WebSocket closed');
    };

    ws.onerror = (event) => {
      if (!active) return;
      setError('WebSocket error occurred');
      console.error('WebSocket error:', event);
    };

    return () => {
      active = false;
      console.log('Closing WebSocket');
      ws.close();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return { drivers, error };
}