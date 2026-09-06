export class MobileSourceError extends Error {
  constructor(message: string, public retryable: boolean) { super(message); }
}
export interface MobileSource { playUrl: string; playType: 'mp4' | 'm3u8' }
export async function resolveMobileSource(id: string, signal?: AbortSignal): Promise<MobileSource> {
  const headers: Record<string, string> = {};
  for (const name of ['token', 'adminToken']) {
    const token = sessionStorage.getItem(name);
    if (token) headers[name] = token;
  }
  const response = await fetch(`/api/play/video/info/${encodeURIComponent(id)}`, { signal, headers });
  if (!response.ok) throw new MobileSourceError(
    response.status === 401 || response.status === 403 ? '没有播放权限，请重新登录后再试' :
      response.status === 404 ? '视频不存在或已被移除' : '播放地址暂不可用', response.status >= 500);
  const result = await response.json();
  if (!result.status || !result.data) throw new MobileSourceError(result.msg || '无法获取视频信息', false);
  const info = result.data.video_basic_info;
  const formats = String(info?.containerFormat || '').toLowerCase().split(',').map(value => value.trim());
  // ffprobe 共用 matroska,webm 标识。H.264/AAC 的 MKV 不是 WebM，
  // 手机 WebView 能起播也不代表能可靠随机定位，交给现有 HLS 接口处理。
  const webmVideo = ['vp8', 'vp9', 'av1'].includes(String(info?.videoCodec || '').toLowerCase());
  const webmAudio = !info?.audioCodec || ['opus', 'vorbis'].includes(String(info.audioCodec).toLowerCase());
  const incompatibleMatroska = (formats.includes('matroska') || formats.includes('webm')) && !(webmVideo && webmAudio);
  const playType = result.data.is_web && !incompatibleMatroska ? 'mp4' : 'm3u8';
  const version = result.data.media_version ? `?mediaVersion=${encodeURIComponent(result.data.media_version)}` : '';
  return { playUrl: `/api/video/${playType}/${encodeURIComponent(id)}/v.${playType}${version}`, playType };
}
