'use client';

import Link from 'next/link';
import { useEffect, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { useTripFeed } from '@/hooks/useTripFeed';
import { formatDistance, formatDuration, formatLocation, tripRequest } from '@/lib/trips';
import { Trip, TripStatus, tripStatusLabels } from '@/types';

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
  const [driverId, setDriverId] = useState('');
  const [online, setOnline] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const { trips, connected, ready, updateTrip } = useTripFeed({ driverId: online ? driverId : '' });
  useEffect(() => {
    const saved = localStorage.getItem('ride-sharing-driver');
    setDriverId(saved && /^driver-[0-3]$/.test(saved) ? saved : 'driver-0');
  }, []);
  const active = trips.find(trip => trip.driver_id === driverId && trip.status >= TripStatus.Accepted && trip.status <= TripStatus.InTrip);
  const available = trips.filter(trip => trip.status === TripStatus.Pending);
  const history = trips.filter(trip => trip.driver_id === driverId && trip.status >= TripStatus.Completed);

  const perform = async (trip: Trip, status: TripStatus) => {
    if (busy) return;
    setBusy(true);
    setError(null);
    try {
      const result = status === TripStatus.Accepted
        ? await tripRequest<Trip>('/trip/accept', 'POST', { trip_id: trip.trip_id, driver_id: driverId })
        : await tripRequest<Trip>(`/trip/${trip.trip_id}/status`, 'PATCH', { driver_id: driverId, status });
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
          <Link href="/" className="text-sm underline">返回乘客端</Link>
        </header>
        <Card>
          <CardContent className="flex flex-wrap items-center gap-4 pt-6">
            <label className="flex items-center gap-2 text-sm">演示司机
              <select aria-label="演示司机" value={driverId} disabled={!!active || busy || !ready && online} onChange={event => {
                setDriverId(event.target.value);
                localStorage.setItem('ride-sharing-driver', event.target.value);
                setError(null);
              }} className="rounded-md border bg-background px-3 py-2">
                {[0, 1, 2, 3].map(id => <option key={id} value={`driver-${id}`}>driver-{id}</option>)}
              </select>
            </label>
            <Button variant="outline" disabled={!!active || busy || !driverId || !ready && online} onClick={() => setOnline(value => !value)}>{online ? '暂停接单' : '开始接单'}</Button>
            <span role="status" className="text-sm text-muted-foreground">{!online ? '已暂停接收订单' : connected ? '实时连接正常' : '正在连接，自动恢复订单…'}</span>
          </CardContent>
        </Card>
        <p className="text-xs text-muted-foreground">演示模式：通过司机 ID 区分身份，尚未接入登录认证。当前车辆位置仍由预设路线模拟。</p>
        {error && <p role="alert" className="rounded-lg border border-destructive p-3 text-sm text-destructive">{error}</p>}
        {active && <Card>
          <CardHeader><CardTitle>当前行程 · {tripStatusLabels[active.status]}</CardTitle><CardDescription>请按实际进度推进，进行中的行程完成后才能接新订单。</CardDescription></CardHeader>
          <CardContent className="space-y-5">
            <TripDetails trip={active} />
            <div className="flex flex-wrap gap-3">
              {nextStep[active.status] && <Button disabled={busy || !connected} onClick={() => perform(active, nextStep[active.status]!.status)}>{busy ? '正在提交…' : nextStep[active.status]!.label}</Button>}
              {active.status < TripStatus.InTrip && <Button variant="outline" disabled={busy || !connected} onClick={() => perform(active, TripStatus.Cancelled)}>取消行程</Button>}
            </div>
          </CardContent>
        </Card>}
        <section className="space-y-4">
          <h2 className="text-xl font-semibold">待接订单 <span className="text-muted-foreground">{available.length}</span></h2>
          {online && ready && available.length === 0 && <p className="rounded-xl border bg-background p-6 text-sm text-muted-foreground">暂无待接订单。在乘客端选择目的地并确认叫车，新订单会自动出现在这里。</p>}
          {!online && <p className="text-sm text-muted-foreground">开始接单后会加载最新订单。</p>}
          <div className="grid gap-4 md:grid-cols-2">
            {available.map(trip => <Card key={trip.trip_id}>
              <CardHeader><CardTitle className="text-base">新行程</CardTitle></CardHeader>
              <CardContent className="space-y-4"><TripDetails trip={trip} /><Button className="w-full" disabled={busy || !!active || !connected} onClick={() => perform(trip, TripStatus.Accepted)}>{active ? '请先完成当前行程' : busy ? '正在提交…' : '接受订单'}</Button></CardContent>
            </Card>)}
          </div>
        </section>
        {history.length > 0 && <section className="space-y-4"><h2 className="text-xl font-semibold">最近订单</h2><div className="grid gap-4 md:grid-cols-2">{history.map(trip => <Card key={trip.trip_id}><CardHeader><CardTitle className="text-base">{tripStatusLabels[trip.status]}</CardTitle></CardHeader><CardContent><TripDetails trip={trip} /></CardContent></Card>)}</div></section>}
      </div>
    </main>
  );
}
