import { flushPromises, mount } from '@vue/test-utils';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { saveMobileSession } from '@/common/mobileSession';
import type { I_resource } from '@/dataType/resource.dataType';
import type { T_resourcesShowMode } from '@/dataType/app.dataType';
import MobileResourceLayout from '../../mobile/MobileResourceLayout.vue';
vi.mock('@/common/photo', () => ({ getResourceCoverPoster: () => '' }));
vi.mock('@/common/videoDuration', () => ({ getResourceDurationText: () => '' }));
vi.mock('@/common/play', () => ({ playUpdate: vi.fn() }));
vi.mock('@/language/app.lang', () => ({ AppLang: () => ({ definition: () => '' }) }));
vi.mock('@/storeData/app.storeData', () => ({ appStoreData: () => ({ appConfig: {}, currentConfigApp: {}, currentFilesBases: { id: 'library' } }) }));
vi.mock('@/components/play/mobileVideoPlayer.vue', async () => {
  const { defineComponent, h } = await import('vue');
  return { __esModule: true, default: defineComponent({
    props: ['resourceId', 'dramaSeriesId', 'autoplay', 'title'],
    setup(props, { expose }) {
      expose({ isPlaying: () => false });
      return () => h('div', { 'data-player': 'xgplayer', 'data-episode': props.dramaSeriesId });
    },
  }) };
});
const resources = ['one', 'two'].map(id => ({ id, title: id, dramaSeries: [{ id: `episode-${id}` }],
  tags: [], coverPosterWidth: 0, coverPosterHeight: 0 })) as unknown as I_resource[];
const inlineModes: T_resourcesShowMode[] = ['coverPosterMosaicShortVideo', 'shortVideo', 'shortVideoTopBottom'];
const detailModes: T_resourcesShowMode[] = ['coverPoster', 'coverPosterCinemaGallery', 'coverPosterSimple',
  'coverPosterSimpleExpand', 'coverPosterBox', 'coverPosterBoxWideSeparate', 'table',
  'coverPosterWaterfall', 'coverPosterMosaic', 'coverPosterCompactWall'];
describe('全部手机显示模式的播放入口', () => {
  beforeEach(() => localStorage.clear());
  it('重载恢复选中的资源，列表未加载时不覆盖记忆', async () => {
    saveMobileSession({ inlineVideo: { filesBasesId: 'library', resourceId: 'two' } });
    const wrapper = mount(MobileResourceLayout, { props: { mode: 'shortVideo', dataList: [] }, global: { stubs: { 'el-icon': true } } });
    await wrapper.setProps({ dataList: resources }); await flushPromises();
    expect(wrapper.find('[data-player="xgplayer"]').attributes('data-episode')).toBe('episode-two');
    wrapper.unmount();
  });
  it.each(inlineModes)('%s 使用手机播放器，切换分集，清空列表停止播放', async mode => {
    const wrapper = mount(MobileResourceLayout, { props: { mode, dataList: resources }, global: { stubs: { 'el-icon': true } } });
    await flushPromises();
    expect(wrapper.find('[data-player="xgplayer"]').attributes('data-episode')).toBe('episode-one');
    await wrapper.findAll('.mobile-short-card')[1].trigger('click');
    expect(wrapper.find('[data-player="xgplayer"]').attributes('data-episode')).toBe('episode-two');
    await wrapper.setProps({ dataList: [] });
    expect(wrapper.find('[data-player="xgplayer"]').exists()).toBe(false);
    wrapper.unmount();
  });
  it.each(detailModes)('%s 通过统一资源选择事件进入播放页', async mode => {
    const wrapper = mount(MobileResourceLayout, { props: { mode, dataList: resources }, global: { stubs: { 'el-icon': true } } });
    await wrapper.find('button').trigger('click');
    expect(wrapper.emitted('select-resource')?.[0]).toEqual([resources[0]]);
    expect(wrapper.find('[data-player="xgplayer"]').exists()).toBe(false);
    expect(wrapper.find('video').exists()).toBe(false);
    wrapper.unmount();
  });
});
