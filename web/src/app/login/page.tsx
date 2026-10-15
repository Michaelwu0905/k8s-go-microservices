'use client';
import { FormEvent, useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/components/AuthProvider';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';

export default function LoginPage() {
  const { user, ready, signIn } = useAuth();
  const router = useRouter();
  const [register, setRegister] = useState(false);
  const [role, setRole] = useState<'rider' | 'driver'>('rider');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => { if (ready && user) router.replace(user.role === 'driver' ? '/driver' : '/'); }, [user, ready, router]);
  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault(); if (busy) return;
    const data = new FormData(event.currentTarget);
    const password = String(data.get('password') ?? '');
    if (register && password !== data.get('confirm')) { setError('两次密码不一致'); return; }
    setBusy(true); setError(null);
    try {
      const body = register
        ? { username: data.get('username'), password, display_name: data.get('display_name'), role }
        : { username: data.get('username'), password };
      const signedIn = await signIn(register ? '/auth/register' : '/auth/login', body);
      router.replace(signedIn.role === 'driver' ? '/driver' : '/');
    } catch (error) { setError(error instanceof Error ? error.message : '登录失败，请重试'); }
    finally { setBusy(false); }
  };
  return <main className="grid min-h-screen place-items-center bg-muted/30 px-4 py-10">
    <div className="w-full max-w-md space-y-5">
      <div><p className="mb-2 text-sm text-muted-foreground">RIDE SHARING</p><h1 className="text-3xl font-semibold">出发前，先登录</h1><p className="mt-2 text-sm text-muted-foreground">用您的账号查看行程、接单与同步位置。</p></div>
      <Card>
        <CardHeader><CardTitle>{register ? '创建账号' : '登录账号'}</CardTitle><CardDescription>{register ? '选择固定身份，注册后自动进入对应页面。' : '登录后自动进入乘客端或司机工作台。'}</CardDescription></CardHeader>
        <CardContent>
          <form onSubmit={submit} className="space-y-4">
            <label className="block space-y-2 text-sm"><span>账号</span><input required name="username" autoComplete="username" minLength={3} maxLength={32} pattern="[a-zA-Z0-9_]{3,32}" placeholder="字母、数字或下划线" className="w-full rounded-md border bg-background px-3 py-2" /></label>
            {register && <>
              <label className="block space-y-2 text-sm"><span>昵称</span><input required name="display_name" maxLength={40} autoComplete="nickname" className="w-full rounded-md border bg-background px-3 py-2" /></label>
              <label className="block space-y-2 text-sm"><span>身份</span><select aria-label="身份" value={role} onChange={event => setRole(event.target.value as 'rider' | 'driver')} className="w-full rounded-md border bg-background px-3 py-2"><option value="rider">乘客</option><option value="driver">司机</option></select></label>
            </>}
            <label className="block space-y-2 text-sm"><span>密码</span><input required type="password" name="password" autoComplete={register ? 'new-password' : 'current-password'} minLength={register ? 10 : 1} maxLength={72} placeholder={register ? '至少 10 位，最多 72 字节' : '输入密码'} className="w-full rounded-md border bg-background px-3 py-2" /></label>
            {register && <label className="block space-y-2 text-sm"><span>确认密码</span><input required type="password" name="confirm" autoComplete="new-password" minLength={10} maxLength={72} className="w-full rounded-md border bg-background px-3 py-2" /></label>}
            {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
            <Button className="w-full" disabled={busy || !ready} type="submit">{busy ? '正在提交…' : register ? '注册并进入' : '登录'}</Button>
            <Button className="w-full" variant="ghost" type="button" disabled={busy} onClick={() => { setRegister(value => !value); setError(null); }}>{register ? '已有账号？去登录' : '没有账号？注册'}</Button>
          </form>
        </CardContent>
      </Card>
      <p className="text-xs text-muted-foreground">同时体验乘客与司机时，请使用不同浏览器或独立的无痕窗口，分别登录两个账号。</p>
    </div>
  </main>;
}
