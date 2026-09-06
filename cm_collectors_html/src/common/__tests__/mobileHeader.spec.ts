import { mount } from '@vue/test-utils';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import MobileHeader from '@/views/MobileHeaderView.vue';
const router = vi.hoisted(() => ({ options: { history: { state: { back: null as string | null } } }, back: vi.fn(), replace: vi.fn(), push: vi.fn() }));
vi.mock('vue-router', () => ({ useRouter: () => router }));
beforeEach(() => { vi.clearAllMocks(); router.options.history.state.back = null; });
describe('手机页面返回', () => {
  async function clickBack() {
    const wrapper = mount(MobileHeader, { global: { stubs: { ElButton: { template: '<button><slot /></button>' } } } });
    await wrapper.find('button').trigger('click'); wrapper.unmount();
  }
  it('刷新后没有上一页时回到资源列表', async () => {
    await clickBack(); expect(router.replace).toHaveBeenCalledWith('/mobile'); expect(router.back).not.toHaveBeenCalled();
  });
  it('保留正常浏览历史后退', async () => {
    router.options.history.state.back = '/mobile'; await clickBack(); expect(router.back).toHaveBeenCalledOnce(); expect(router.replace).not.toHaveBeenCalled();
  });
  it('不返回旧版桥接产生的地址', async () => {
    router.options.history.state.back = '/__cm_shell__/theme/dark'; await clickBack(); expect(router.replace).toHaveBeenCalledWith('/mobile');
  });
});
