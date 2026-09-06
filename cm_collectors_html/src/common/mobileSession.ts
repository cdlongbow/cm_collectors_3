import type { I_searchData } from '@/dataType/search.dataType';

const sessionKey = 'cm-phone-session-v1';
const playbackKey = 'cm-phone-playback-v1';
const soundKey = 'cm-phone-sound-v1';
export interface MobileSound { muted: boolean; volume: number }
export function readMobileSound(): MobileSound {
  const sound = read<MobileSound>(soundKey);
  return {
    muted: sound?.muted === true,
    volume: typeof sound?.volume === 'number' && Number.isFinite(sound.volume)
      ? Math.max(0, Math.min(1, sound.volume)) : 0.6,
  };
}
export function saveMobileSound(sound: MobileSound) {
  if (typeof sound.muted !== 'boolean' || !Number.isFinite(sound.volume)) return;
  write(soundKey, { muted: sound.muted, volume: Math.max(0, Math.min(1, sound.volume)) });
}
export interface MobileListState {
  filesBasesId: string;
  searchKey: string;
  page: number;
  pageSize: number;
  scrollTop: number;
  randomSeed: string;
  searchText: string;
}
export interface MobileSession {
  path?: string;
  filesBasesId?: string;
  search?: I_searchData;
  list?: MobileListState;
  inlineVideo?: { filesBasesId: string; resourceId: string };
  updatedAt?: number;
}
export interface MobilePlayback {
  time: number;
  duration: number;
  rate: number;
  playing: boolean;
  updatedAt: number;
}
function read<T>(key: string): T | undefined {
  try { return JSON.parse(localStorage.getItem(key) || 'null') || undefined; }
  catch { return undefined; }
}
function write(key: string, value: unknown) {
  try { localStorage.setItem(key, JSON.stringify(value)); }
  catch { /* 禁用存储或空间不足不能阻断播放。 */ }
}
export function isRestorableMobilePath(path: string): boolean {
  return typeof path === 'string' && /^\/mobile(?:[?#]|$)|^\/play\/(movies|comic|atlas)Mobile\/[^/?#]+(?:\/[^/?#]+)?(?:[?#]|$)/.test(path);
}
export function readMobileSession(): MobileSession {
  const value = read<MobileSession>(sessionKey);
  return value && typeof value === 'object' && !Array.isArray(value) ? value : {};
}
export function saveMobileSession(value: Partial<MobileSession>) {
  write(sessionKey, { ...readMobileSession(), ...value, updatedAt: Date.now() });
}
export function mobileRestorePath(entryPath: string, savedPath?: string) {
  // 显式打开的深链接优先；只有壳重新加载入口时才恢复旧页面。
  return (entryPath === '/' || entryPath === '/mobile') && savedPath && isRestorableMobilePath(savedPath)
    ? savedPath : entryPath;
}
export function readMobilePlayback(id: string): MobilePlayback | undefined {
  const item = read<Record<string, MobilePlayback>>(playbackKey)?.[id];
  return item && Number.isFinite(item.time) && item.time >= 0 && Number.isFinite(item.duration)
    && Number.isFinite(item.rate) && item.rate > 0 ? item : undefined;
}
export function resumeTime(item?: MobilePlayback): number {
  if (!item || item.time < 1 || (item.duration > 0 && item.duration - item.time < 5)) return 0;
  return item.time;
}
export function saveMobilePlayback(id: string, value: Omit<MobilePlayback, 'updatedAt'>) {
  if (!id || !Number.isFinite(value.time) || !Number.isFinite(value.duration)) return;
  const stored = read<Record<string, MobilePlayback>>(playbackKey);
  const all = stored && typeof stored === 'object' && !Array.isArray(stored) ? stored : {};
  all[id] = { ...value, updatedAt: Date.now() };
  const entries = Object.entries(all).filter(([, item]) => item && Number.isFinite(item.updatedAt))
    .sort((a, b) => b[1].updatedAt - a[1].updatedAt).slice(0, 100);
  write(playbackKey, Object.fromEntries(entries));
}
