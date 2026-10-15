import { test } from 'node:test';
import assert from 'node:assert/strict';
import { advanceAlongRoute, distanceMeters, remainingRoute, routeLength } from '../src/lib/geo.ts';
const start = { latitude: 39.9, longitude: 116.4 };
const middle = { latitude: 39.901, longitude: 116.4 };
const end = { latitude: 39.901, longitude: 116.402 };
const path = [start, middle, end];
test('demo advances along road segments and stops at the destination', () => {
 let position = start;
 let remaining = routeLength(path);
 for (let i = 0; i < 20; i++) {
  const next = advanceAlongRoute(position, path, 25);
  assert.ok(distanceMeters(position, next) <= 25.1);
  const after = routeLength(remainingRoute(next, path));
  assert.ok(after <= remaining + 0.01);
  remaining = after; position = next;
 }
 assert.ok(distanceMeters(position, end) < 0.01);
});
test('a new road route does not teleport a remote vehicle', () => {
 const remote = { latitude: 31.2, longitude: 121.4 };
 assert.deepEqual(advanceAlongRoute(remote, path, 25), remote);
 assert.deepEqual(advanceAlongRoute(start, [], 25), start);
});
test('remaining route removes traversed segments without reversing', () => {
 const position = { latitude: 39.901, longitude: 116.401 };
 const remaining = remainingRoute(position, path);
 assert.equal(remaining.length, 2);
 assert.ok(distanceMeters(position, remaining[0]) < 0.01);
 assert.ok(routeLength(remaining) < routeLength(path));
});
test('demo joins a nearby snapped road gradually instead of getting stuck', () => {
 const offRoad = { latitude: 39.899, longitude: 116.398 };
 const next = advanceAlongRoute(offRoad, path, 25);
 assert.ok(distanceMeters(offRoad, next) > 24.9);
 assert.ok(distanceMeters(offRoad, next) < 25.1);
 assert.ok(distanceMeters(next, start) < distanceMeters(offRoad, start));
});
