export interface RequestRideProps {
    pickup: [number, number],
    destination: [number, number],
}

export interface Location {
    latitude: number,
    longitude: number,
}

export interface RouteInfo {
    trip_id: string
    status: number
    route: {
        geometry: {
            coordinates: Location[]
        }[]
        distance: number
        duration: number
    }
}