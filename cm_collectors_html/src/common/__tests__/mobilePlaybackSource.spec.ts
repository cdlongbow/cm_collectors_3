import { afterEach, describe, expect, it, vi } from 'vitest';
import { resolveMobileSource } from '../mobilePlaybackSource';

describe('手机播放源错误分类', () => {
  afterEach(() => vi.unstubAllGlobals());
  it.each([401, 403, 404, 500, 503])('HTTP %s 仅服务器错误自动重试', async status => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status }));
    await expect(resolveMobileSource('episode')).rejects.toMatchObject({ retryable: status >= 500 });
  });
  it('保留媒体版本，非浏览器直播放源使用 HLS', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => ({
      status: true, data: { is_web: false, media_version: 'v 2' },
    }) }));
    await expect(resolveMobileSource('episode')).resolves.toEqual({
      playType: 'm3u8', playUrl: '/api/video/m3u8/episode/v.m3u8?mediaVersion=v%202',
    });
  });
  it.each([
    ['matroska,webm', 'h264', 'aac', 'm3u8'],
    ['matroska,webm', 'hevc', 'aac', 'm3u8'],
    ['matroska,webm', 'vp9', 'aac', 'm3u8'],
    ['matroska,webm', 'vp9', 'opus', 'mp4'],
    ['webm', 'vp8', 'vorbis', 'mp4'],
    ['matroska,webm', 'av1', '', 'mp4'],
    ['mov,mp4,m4a,3gp,3g2,mj2', 'h264', 'aac', 'mp4'],
  ])('手机播放 %s / %s / %s 使用 %s 路径', async (containerFormat, videoCodec, audioCodec, playType) => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => ({
      status: true, data: { is_web: true, media_version: 'large-4k', video_basic_info: {
        containerFormat, videoCodec, audioCodec, width: 3840, height: 2160, frameRate: 59.94,
      } },
    }) }));
    await expect(resolveMobileSource('episode')).resolves.toEqual({
      playType, playUrl: `/api/video/${playType}/episode/v.${playType}?mediaVersion=large-4k`,
    });
  });
  it('旧服务未提供容器信息时保持原播放选择', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => ({
      status: true, data: { is_web: true },
    }) }));
    await expect(resolveMobileSource('episode')).resolves.toEqual({
      playType: 'mp4', playUrl: '/api/video/mp4/episode/v.mp4',
    });
  });

});
