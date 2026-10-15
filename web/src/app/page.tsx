'use client';

import dynamic from 'next/dynamic';
import { RequireAuth } from '@/components/AuthProvider';

const NearbyDriversMap = dynamic(
  () => import('../components/NearbyDriversMap').then((mod) => mod.NearbyDriversMap),
  { ssr: false }
);

export default function Home() {
  return (
    <RequireAuth role="rider"><main className="h-full w-full">
      <NearbyDriversMap />
    </main></RequireAuth>
  );
}
