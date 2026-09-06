import { beforeEach, describe, expect, it } from 'vitest';
import { isRestorableMobilePath, mobileRestorePath, readMobilePlayback, readMobileSession, resumeTime, saveMobilePlayback, saveMobileSession } from '../mobileSession';
import { parseMobileVtt } from '../mobileSubtitles';

describe('手机状态恢复', () => {
  beforeEach(() => localStorage.clear());
  it('壳加载首页时恢复播放页，显式深链接不被旧状态覆盖', () => {
    expect(mobileRestorePath('/mobile', '/play/moviesMobile/r/e')).toBe('/play/moviesMobile/r/e');
    expect(mobileRestorePath('/play/moviesMobile/new/e', '/play/moviesMobile/old/e')).toBe('/play/moviesMobile/new/e');
  });
  it('不恢复管理页或外部地址，主动回首页会更新恢复目标', () => {
    expect(isRestorableMobilePath('//example.com')).toBe(false);
    expect(isRestorableMobilePath('/setting')).toBe(false);
    saveMobileSession({ path: '/play/moviesMobile/r/e', filesBasesId: 'library' });
    saveMobileSession({ path: '/mobile' });
    expect(readMobileSession()).toMatchObject({ path: '/mobile', filesBasesId: 'library' });
  });
  it('损坏存储可正常启动，分集进度独立，接近结尾从头播放', () => {
    localStorage.setItem('cm-phone-session-v1', '{broken');
    expect(readMobileSession()).toEqual({});
    saveMobilePlayback('r/e1', { time: 95, duration: 100, rate: 1.5, playing: false });
    saveMobilePlayback('r/e2', { time: 99, duration: 100, rate: 1, playing: true });
    expect(resumeTime(readMobilePlayback('r/e1'))).toBe(95);
    expect(resumeTime(readMobilePlayback('r/e2'))).toBe(0);
    expect(readMobilePlayback('r/e1')?.rate).toBe(1.5);
  });
  it('VTT 字幕安全显示文本，支持编号、定位属性和多行', () => {
    expect(parseMobileVtt('WEBVTT\n\n1\n00:00:01.000 --> 00:00:03.500 align:center\n<b>你好</b> &amp;\n世界')).toEqual([
      { start: 1, end: 3.5, text: '你好 &\n世界' },
    ]);
  });
});
