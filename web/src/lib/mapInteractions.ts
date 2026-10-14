import { DomEvent } from 'leaflet';

// Leaflet handles native events before React's delegated event handlers run.
// Use this as a ref on map overlays so their controls do not interact with the map.
export function disableMapPropagation(element: HTMLElement | null): void {
  if (!element) return;

  DomEvent.disableClickPropagation(element);
  DomEvent.disableScrollPropagation(element);
}
