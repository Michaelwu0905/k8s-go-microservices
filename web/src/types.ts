export interface Location {
  latitude: number;
  longitude: number;
}

export enum TripStatus {
  Pending = 1,
  Accepted = 2,
  Arrived = 3,
  InTrip = 4,
  Completed = 5,
  Cancelled = 6,
}

export interface Trip {
  trip_id: string;
  status: TripStatus;
  route: {
    geometry: { coordinates: Location[] }[];
    distance: number;
    duration: number;
  };
  pickup: Location;
  destination: Location;
  driver_id?: string;
  rider_id: string;
  created_at: string;
  updated_at: string;
  version: number;
}

export const tripStatusLabels: Record<TripStatus, string> = {
  [TripStatus.Pending]: '等待司机接单',
  [TripStatus.Accepted]: '司机正在接驾',
  [TripStatus.Arrived]: '司机已到达',
  [TripStatus.InTrip]: '行程进行中',
  [TripStatus.Completed]: '行程已完成',
  [TripStatus.Cancelled]: '订单已取消',
};

export function isActiveTrip(trip: Trip): boolean {
  return trip.status >= TripStatus.Pending && trip.status <= TripStatus.InTrip;
}

export interface Driver {
  driver_id: string;
  location: Location;
  geohash?: string;
  online?: boolean;
  updated_at?: string;
  source?: 'gps' | 'demo';
  accuracy?: number;
  distance_meters?: number;
}

export interface Navigation {
  trip_id: string;
  phase: 'pickup' | 'destination';
  route: Trip['route'];
  driver: Driver;
}
