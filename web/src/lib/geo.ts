import type { Location } from '@/types';

const radians = Math.PI / 180;
export function distanceMeters(a: Location, b: Location): number {
  const lat = (b.latitude - a.latitude) * radians;
  const lng = (b.longitude - a.longitude) * radians;
  const value = Math.sin(lat / 2) ** 2 + Math.cos(a.latitude * radians) * Math.cos(b.latitude * radians) * Math.sin(lng / 2) ** 2;
  return 6371000 * 2 * Math.asin(Math.sqrt(Math.min(1, value)));
}

function interpolate(a: Location, b: Location, fraction: number): Location {
  return { latitude: a.latitude + (b.latitude - a.latitude) * fraction, longitude: a.longitude + (b.longitude - a.longitude) * fraction };
}

export function routeLength(path: Location[]): number {
  return path.slice(1).reduce((total, point, i) => total + distanceMeters(path[i], point), 0);
}

// Project onto the closest road segment, then retain only the route ahead.
export function remainingRoute(position: Location, path: Location[]): Location[] {
  if (path.length < 2) return path;
  let bestDistance = Infinity;
  let bestIndex = 0;
  let projection = path[0];
  const longitudeScale = Math.cos(position.latitude * radians);
  for (let i = 0; i < path.length - 1; i++) {
    const a = path[i], b = path[i + 1];
    const x = (b.longitude - a.longitude) * longitudeScale;
    const y = b.latitude - a.latitude;
    const px = (position.longitude - a.longitude) * longitudeScale;
    const py = position.latitude - a.latitude;
    const fraction = x * x + y * y === 0 ? 0 : Math.max(0, Math.min(1, (px * x + py * y) / (x * x + y * y)));
    const point = interpolate(a, b, fraction);
    const distance = distanceMeters(position, point);
    if (distance < bestDistance) { bestDistance = distance; bestIndex = i; projection = point; }
  }
  return [projection, ...path.slice(bestIndex + 1)];
}

export function advanceAlongRoute(position: Location, path: Location[], meters: number): Location {
  if (path.length < 2 || meters <= 0) return position;
  const remaining = remainingRoute(position, path);
  // OSRM snaps the origin to a drivable road. Join a nearby snapped origin
  // gradually, including when a GPS fix is inside a park or courtyard.
  const gap = distanceMeters(position, remaining[0]);
  if (gap > 500) return position;
  if (gap > meters) return interpolate(position, remaining[0], meters / gap);
  let budget = meters - gap;
  for (let i = 1; i < remaining.length; i++) {
    const length = distanceMeters(remaining[i - 1], remaining[i]);
    if (length > budget) return interpolate(remaining[i - 1], remaining[i], budget / length);
    budget -= length;
  }
  return remaining[remaining.length - 1];
}
