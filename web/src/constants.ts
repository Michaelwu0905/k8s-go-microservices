export const API_URL = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8081';
export const WEBSOCKET_URL = process.env.NEXT_PUBLIC_WEBSOCKET_URL ?? `${API_URL.replace(/^http/, 'ws')}/ws`;
