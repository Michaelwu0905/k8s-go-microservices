'use client';

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { disableMapPropagation } from '@/lib/mapInteractions';
import { formatDistance, formatDuration, formatLocation } from '@/lib/trips';
import { Driver, Location, Navigation, Trip, TripStatus, tripStatusLabels } from '@/types';

interface RideConfirmationCardProps {
  trip: Trip | null;
  driver?: Driver;
  navigation: Navigation | null;
  remainingDistance: number | null;
  remainingDuration: number | null;
  navigationError: string | null;
  pickup: Location | null;
  destination: Location | null;
  selectionTarget: 'pickup' | 'destination';
  canEdit: boolean;
  onSelectTarget: (target: 'pickup' | 'destination') => void;
  busy: boolean;
  error: string | null;
  canRequest: boolean;
  onRequest: () => void;
  onCancel: () => void;
  onReset: () => void;
}

export function RideConfirmationCard({ trip, driver, navigation, remainingDistance, remainingDuration, navigationError, pickup, destination, selectionTarget, canEdit, onSelectTarget, busy, error, canRequest, onRequest, onCancel, onReset }: RideConfirmationCardProps) {
  const terminal = trip && [TripStatus.Completed, TripStatus.Cancelled].includes(trip.status);
  return (
    <Card ref={disableMapPropagation} className="absolute bottom-4 right-4 z-[1000] max-h-[calc(100dvh-10rem)] w-[calc(100%-2rem)] max-w-sm overflow-y-auto">
      <CardHeader>
        <CardTitle>{trip ? tripStatusLabels[trip.status] : '设置起点和终点'}</CardTitle>
        <CardDescription>
          {trip ? `订单 ${trip.trip_id.slice(0, 8)}` : busy ? '正在提交所选位置，请稍候。' : !canEdit ? '正在恢复订单，请稍候。' : `点击地图设置${selectionTarget === 'pickup' ? '起点' : '终点'}，也可切换下方选点目标。`}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="space-y-2">
          {(['pickup', 'destination'] as const).map(target => {
            const point = target === 'pickup' ? pickup : destination;
            const label = target === 'pickup' ? '起点' : '终点';
            const content = <>
              <span aria-hidden="true" className={`mt-1.5 h-2.5 w-2.5 shrink-0 rounded-full ${target === 'pickup' ? 'bg-green-600' : 'bg-blue-600'}`} />
              <span className="min-w-0">
                <span className="block font-medium">{trip ? label : `设置${label}`}</span>
                <span aria-label={`${label}位置`} className="block text-xs font-normal text-muted-foreground">{point ? formatLocation(point) : '尚未选择'}</span>
              </span>
            </>;
            return trip ? (
              <div key={target} className="flex items-start gap-3 rounded-md border px-3 py-2 text-sm">{content}</div>
            ) : (
              <Button key={target} aria-label={`设置${label}`} aria-pressed={selectionTarget === target}
                variant={selectionTarget === target ? 'secondary' : 'outline'} disabled={!canEdit || busy}
                className={`h-auto w-full justify-start gap-3 whitespace-normal py-2 text-left ${selectionTarget === target ? 'ring-1 ring-foreground/30' : ''}`}
                onClick={() => onSelectTarget(target)}>{content}</Button>
            );
          })}
        </div>
        {trip ? (
          <>
            <p className="break-all text-sm">{trip.driver_id ? `服务司机：${trip.driver_id}` : '订单已发布，请在司机端接单。'}</p>
            {trip.driver_id && trip.status <= TripStatus.InTrip && <>
              <p className="text-xs text-muted-foreground">{driver?.online ? `${driver.source === 'gps' ? '设备定位' : '演示行驶'} · 位置实时更新` : '司机位置暂不可用，正在等待更新'}</p>
              {navigation && driver?.online && <p className="text-sm font-medium">{navigation.phase === 'pickup' ? '接驾剩余' : '距目的地'} {formatDistance(remainingDistance ?? 0)} · {formatDuration(remainingDuration ?? 0)}</p>}
              {navigationError && <p className="text-xs text-destructive">接驾/导航路线暂不可用，请稍后重试。</p>}
            </>}
            {trip.status === TripStatus.Arrived && <p className="text-sm">司机已到上车点，请准备上车。</p>}
            <div className="grid grid-cols-2 gap-3 text-sm">
              <div><p className="text-muted-foreground">行程距离</p><p>{formatDistance(trip.route.distance)}</p></div>
              <div><p className="text-muted-foreground">预计时长</p><p>{formatDuration(trip.route.duration)}</p></div>
            </div>
            <p className="text-xs text-muted-foreground">状态更新于 {new Date(trip.updated_at).toLocaleTimeString('zh-CN')}</p>
          </>
        ) : null}
        {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
        {!trip && <Button disabled={busy || !canRequest} className="w-full" onClick={onRequest}>{busy ? '正在创建订单…' : '确认叫车'}</Button>}
        {trip && trip.status <= TripStatus.Arrived && <Button variant="outline" disabled={busy} className="w-full" onClick={onCancel}>{busy ? '正在取消…' : '取消订单'}</Button>}
        {terminal && <Button className="w-full" onClick={onReset}>再叫一辆</Button>}
      </CardContent>
    </Card>
  );
}
