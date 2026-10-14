'use client';

import { useState } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Driver } from '@/hooks/useNearbyDrivers';
import { API_URL } from '@/constants';
import { disableMapPropagation } from '@/lib/mapInteractions';

interface RideConfirmationCardProps {
  driver: Driver;
  tripId: string;
  distance: number; // 米
  duration: number; // 秒
  onCancel: () => void;
}

type OrderStatus = 'confirming' | 'requesting' | 'accepted';

function formatDistance(meters: number): string {
  if (meters < 1000) return `${Math.round(meters)} 米`;
  return `${(meters / 1000).toFixed(1)} 公里`;
}

function formatDuration(seconds: number): string {
  const minutes = Math.round(seconds / 60);
  if (minutes < 1) return '1 分钟内';
  return `约 ${minutes} 分钟`;
}

function calcPrice(meters: number): string {
  const km = meters / 1000;
  const baseFare = 13; // 起步价（含 3 公里）
  const perKm = 2.3;
  const price = km <= 3 ? baseFare : baseFare + (km - 3) * perKm;
  return `¥${price.toFixed(1)}`;
}

export function RideConfirmationCard({ driver, tripId, distance, duration, onCancel }: RideConfirmationCardProps) {
  const [status, setStatus] = useState<OrderStatus>('confirming');
  const [error, setError] = useState<string | null>(null);

  const handleConfirm = async () => {
    setStatus('requesting');
    setError(null);
    try {
      const resp = await fetch(`${API_URL}/trip/accept`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ trip_id: tripId, driver_id: driver.driver_id }),
      });
      if (!resp.ok) {
        throw new Error(`请求失败: HTTP ${resp.status}`);
      }
      setStatus('accepted');
    } catch (e) {
      setError(e instanceof Error ? e.message : '请求失败');
      setStatus('confirming');
    }
  };

  if (status === 'requesting') {
    return (
      <Card ref={disableMapPropagation} className="w-full max-w-sm z-[9999] absolute bottom-4 right-4">
        <CardHeader>
          <CardTitle>正在为您匹配司机</CardTitle>
          <CardDescription>{driver.driver_id} 正在接单…</CardDescription>
        </CardHeader>
        <CardContent className="flex items-center justify-center py-6">
          <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
        </CardContent>
      </Card>
    );
  }

  if (status === 'accepted') {
    return (
      <Card ref={disableMapPropagation} className="w-full max-w-sm z-[9999] absolute bottom-4 right-4">
        <CardHeader>
          <CardTitle>司机已接单 ✓</CardTitle>
          <CardDescription>{driver.driver_id} 正在赶来接您</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="grid grid-cols-3 gap-2 text-sm">
            <div>
              <p className="text-muted-foreground">距离</p>
              <p className="font-semibold">{formatDistance(distance)}</p>
            </div>
            <div>
              <p className="text-muted-foreground">预计时长</p>
              <p className="font-semibold">{formatDuration(duration)}</p>
            </div>
            <div>
              <p className="text-muted-foreground">预估价格</p>
              <p className="font-semibold text-primary">{calcPrice(distance)}</p>
            </div>
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card ref={disableMapPropagation} className="w-full max-w-sm z-[9999] absolute bottom-4 right-4">
      <CardHeader>
        <CardTitle>确认行程</CardTitle>
        <CardDescription>司机 {driver.driver_id} · Toyota Camry</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid grid-cols-3 gap-2 text-sm">
          <div>
            <p className="text-muted-foreground">距离</p>
            <p className="font-semibold">{formatDistance(distance)}</p>
          </div>
          <div>
            <p className="text-muted-foreground">预计时长</p>
            <p className="font-semibold">{formatDuration(duration)}</p>
          </div>
          <div>
            <p className="text-muted-foreground">预估价格</p>
            <p className="font-semibold text-primary">{calcPrice(distance)}</p>
          </div>
        </div>
        {error && <p className="text-sm text-destructive">{error}</p>}
        <div className="flex gap-2">
          <Button variant="outline" className="flex-1" onClick={onCancel}>取消</Button>
          <Button className="flex-1" onClick={handleConfirm}>确认叫车</Button>
        </div>
      </CardContent>
    </Card>
  );
}
