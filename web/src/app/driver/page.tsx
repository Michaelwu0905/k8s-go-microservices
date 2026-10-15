'use client';

import { RequireAuth, useAuth } from '@/components/AuthProvider';
import { AccountMenu } from '@/components/AccountMenu';
import dynamic from 'next/dynamic';
import { useDriverTracking } from '@/hooks/useDriverTracking';
import { useNavigation } from '@/hooks/useNavigation';
import { useEffect, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { useTripFeed } from '@/hooks/useTripFeed';
import { formatDistance, formatDuration, formatLocation, tripRequest } from '@/lib/trips';
import { Trip, TripStatus, tripStatusLabels } from '@/types';

const DriverMap = dynamic(() => import('@/components/DriverMap').then(module => module.DriverMap), { ssr: false });

const nextStep: Partial<Record<TripStatus, { status: TripStatus; label: string }>> = {
  [TripStatus.Accepted]: { status: TripStatus.Arrived, label: '已到达上车点' },
  [TripStatus.Arrived]: { status: TripStatus.InTrip, label: '乘客已上车，开始行程' },
  [TripStatus.InTrip]: { status: TripStatus.Completed, label: '到达目的地，结束行程' },
};

function TripDetails({ trip }: { trip: Trip }) {
  return (
    <div className="space-y-2 text-sm">
      <p><span className="text-muted-foreground">上车点：</span>{formatLocation(trip.pickup)}</p>
      <p><span className="text-muted-foreground">目的地：</span>{formatLocation(trip.destination)}</p>
      <p>{formatDistance(trip.route.distance)} · {formatDuration(trip.route.duration)}</p>
      <p className="text-xs text-muted-foreground">订单 {trip.trip_id.slice(0, 8)} · {new Date(trip.created_at).toLocaleString('zh-CN')}</p>
    </div>
  );
}

export default function DriverPage() {
  return <RequireAuth role="driver"><DriverDashboard /></RequireAuth>;
}
function DriverDashboard() {
  const { user } = useAuth();
  const driverId = user?.id ?? '';
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const { trips, connected, ready, updateTrip } = useTripFeed({ driverId });

  const active = trips.find(trip => trip.driver_id === driverId && trip.status >= TripStatus.Accepted && trip.status <= TripStatus.InTrip);
  const available = trips.filter(trip => trip.status === TripStatus.Pending);
  const history = trips.filter(trip => trip.driver_id === driverId && trip.status >= TripStatus.Completed);
  const tracking = useDriverTracking(driverId);
  const navigation = useNavigation(active, tracking.driver);
  const { setPath } = tracking;
  useEffect(() => {
    setPath(active && [TripStatus.Accepted, TripStatus.InTrip].includes(active.status) ? navigation.path : []);
  }, [active, navigation.path, setPath]);
  const canAccept = tracking.mode !== 'idle' && !!tracking.driver?.online && !tracking.error;


  const perform = async (trip: Trip, status: TripStatus) => {
    if (busy) return;
    setBusy(true);
    setError(null);
    try {
      const result = status === TripStatus.Accepted
        ? await tripRequest<Trip>('/trip/accept', 'POST', { trip_id: trip.trip_id })
        : await tripRequest<Trip>(`/trip/${trip.trip_id}/status`, 'PATCH', { status });
      updateTrip(result);
    } catch (error) {
      setError(error instanceof Error ? error.message : '操作失败，请重试');
    } finally { setBusy(false); }
  };

  return (
    <main className="min-h-screen bg-muted/30 px-4 py-8 sm:px-8">
      <div className="mx-auto max-w-5xl space-y-6">
        <header className="flex flex-wrap items-start justify-between gap-4">
          <div><p className="mb-2 text-sm text-muted-foreground">RIDE SHARING</p><h1 className="text-3xl font-semibold">司机工作台</h1><p className="mt-2 text-sm text-muted-foreground">接单、接驾与行程状态会实时同步给乘客。</p></div>
          <AccountMenu />
        </header>
        <Card>
          <CardContent className="flex flex-wrap items-center gap-4 pt-6">
            <span className="text-sm font-medium">{user?.display_name} · 司机</span>
            <Button disabled={!driverId || tracking.mode === 'demo'} onClick={() => tracking.setMode('demo')}>演示行驶</Button>
            <Button variant="outline" disabled={!driverId || tracking.mode === 'gps'} onClick={() => tracking.setMode('gps')}>使用设备定位</Button>
            {tracking.mode !== 'idle' && <Button variant="outline" onClick={tracking.stop}>停止定位</Button>}
            <span role="status" className="text-sm text-muted-foreground">{connected ? '订单实时连接正常' : '正在恢复订单…'}</span>

          </CardContent>
        </Card>
        <p className="text-xs text-muted-foreground">您只能操作当前账号的车辆与订单。设备定位使用真实位置；演示行驶会沿接驾和行程路线前进。</p>
        <Card>
          <CardHeader><CardTitle className="text-lg">车辆位置 · {tracking.mode === 'gps' ? '设备定位' : tracking.mode === 'demo' ? '演示行驶' : '未开启定位'}</CardTitle>
            <CardDescription>{tracking.loading ? '正在获取并上报位置…' : tracking.driver ? `${formatLocation(tracking.driver.location)} · ${tracking.driver.online ? '在线' : '离线或位置过期'}` : '开启定位后，附近乘客可在地图上看到您的车辆。'}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            {tracking.error && <p role="alert" className="text-sm text-destructive">{tracking.error}</p>}
            {tracking.driver?.source === 'gps' && <p className="text-xs text-muted-foreground">定位精度约 {Math.round(tracking.driver.accuracy ?? 0)} 米</p>}
            <DriverMap driver={tracking.driver} trip={active} path={navigation.path} phase={navigation.navigation?.phase} />
          </CardContent>
        </Card>
        {error && <p role="alert" className="rounded-lg border border-destructive p-3 text-sm text-destructive">{error}</p>}
        {active && <Card>
          <CardHeader><CardTitle>当前行程 · {tripStatusLabels[active.status]}</CardTitle><CardDescription>请按实际进度推进，进行中的行程完成后才能接新订单。</CardDescription></CardHeader>
          <CardContent className="space-y-5">
            <TripDetails trip={active} />
            {navigation.navigation && <p className="text-sm font-medium">{navigation.navigation.phase === 'pickup' ? '接驾剩余' : '距目的地'} {formatDistance(navigation.distance ?? 0)} · {formatDuration(navigation.duration ?? 0)}</p>}
            {navigation.error && <p className="text-sm text-destructive">路线暂不可用：{navigation.error}</p>}
            {tracking.mode === 'demo' && <p className="text-xs text-muted-foreground">演示车辆自动沿路线行驶，抵达后请手动确认行程状态。</p>}
            <div className="flex flex-wrap gap-3">
              {nextStep[active.status] && <Button disabled={busy || !connected} onClick={() => perform(active, nextStep[active.status]!.status)}>{busy ? '正在提交…' : nextStep[active.status]!.label}</Button>}
              {active.status < TripStatus.InTrip && <Button variant="outline" disabled={busy || !connected} onClick={() => perform(active, TripStatus.Cancelled)}>取消行程</Button>}
            </div>
          </CardContent>
        </Card>}
        <section className="space-y-4">
          <h2 className="text-xl font-semibold">待接订单 <span className="text-muted-foreground">{available.length}</span></h2>
          {ready && available.length === 0 && <p className="rounded-xl border bg-background p-6 text-sm text-muted-foreground">暂无待接订单。在乘客端选择目的地并确认叫车，新订单会自动出现在这里。</p>}
          {!canAccept && <p className="text-sm text-muted-foreground">请先开启设备定位或演示行驶，再接受新订单。</p>}
          <div className="grid gap-4 md:grid-cols-2">
            {available.map(trip => <Card key={trip.trip_id}>
              <CardHeader><CardTitle className="text-base">新行程</CardTitle></CardHeader>
              <CardContent className="space-y-4"><TripDetails trip={trip} /><Button className="w-full" disabled={busy || !!active || !connected || !canAccept} onClick={() => perform(trip, TripStatus.Accepted)}>{active ? '请先完成当前行程' : !canAccept ? '先开启定位' : busy ? '正在提交…' : '接受订单'}</Button></CardContent>
            </Card>)}
          </div>
        </section>
        {history.length > 0 && <section className="space-y-4"><h2 className="text-xl font-semibold">最近订单</h2><div className="grid gap-4 md:grid-cols-2">{history.map(trip => <Card key={trip.trip_id}><CardHeader><CardTitle className="text-base">{tripStatusLabels[trip.status]}</CardTitle></CardHeader><CardContent><TripDetails trip={trip} /></CardContent></Card>)}</div></section>}
      </div>
    </main>
  );
}
