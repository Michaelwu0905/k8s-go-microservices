'use client';

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { disableMapPropagation } from '@/lib/mapInteractions';
import { formatDistance, formatDuration, formatLocation } from '@/lib/trips';
import { Location, Trip, TripStatus, tripStatusLabels } from '@/types';

interface RideConfirmationCardProps {
  trip: Trip | null;
  destination: Location | null;
  busy: boolean;
  error: string | null;
  canRequest: boolean;
  onRequest: () => void;
  onCancel: () => void;
  onReset: () => void;
}

export function RideConfirmationCard({ trip, destination, busy, error, canRequest, onRequest, onCancel, onReset }: RideConfirmationCardProps) {
  const terminal = trip && [TripStatus.Completed, TripStatus.Cancelled].includes(trip.status);
  return (
    <Card ref={disableMapPropagation} className="absolute bottom-4 right-4 z-[1000] w-[calc(100%-2rem)] max-w-sm">
      <CardHeader>
        <CardTitle>{trip ? tripStatusLabels[trip.status] : destination ? '确认叫车' : '选择目的地'}</CardTitle>
        <CardDescription>
          {trip ? `订单 ${trip.trip_id.slice(0, 8)}` : '点击地图选择目的地，上车点为天安门附近。'}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {trip ? (
          <>
            <p className="text-sm">{trip.driver_id ? `服务司机：${trip.driver_id}` : '订单已发布，请在司机端接单。'}</p>
            {trip.status === TripStatus.Arrived && <p className="text-sm">司机已到上车点，请准备上车。</p>}
            <div className="grid grid-cols-2 gap-3 text-sm">
              <div><p className="text-muted-foreground">行程距离</p><p>{formatDistance(trip.route.distance)}</p></div>
              <div><p className="text-muted-foreground">预计时长</p><p>{formatDuration(trip.route.duration)}</p></div>
            </div>
            <p className="text-xs text-muted-foreground">状态更新于 {new Date(trip.updated_at).toLocaleTimeString('zh-CN')}</p>
          </>
        ) : destination && <p className="text-sm">目的地：{formatLocation(destination)}</p>}
        {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
        {!trip && destination && <Button disabled={busy || !canRequest} className="w-full" onClick={onRequest}>{busy ? '正在创建订单…' : '确认叫车'}</Button>}
        {trip && trip.status <= TripStatus.Arrived && <Button variant="outline" disabled={busy} className="w-full" onClick={onCancel}>{busy ? '正在取消…' : '取消订单'}</Button>}
        {terminal && <Button className="w-full" onClick={onReset}>再叫一辆</Button>}
      </CardContent>
    </Card>
  );
}
