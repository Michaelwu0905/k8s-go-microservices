'use client';

import dynamic from 'next/dynamic';

const NearbyDriversMap = dynamic(
  () => import('../components/NearbyDriversMap').then((mod) => mod.NearbyDriversMap),
  { ssr: false }
);

export default function Home() {
  return (
    <main className="h-full w-full">
      <NearbyDriversMap />
    </main>
  );
}
