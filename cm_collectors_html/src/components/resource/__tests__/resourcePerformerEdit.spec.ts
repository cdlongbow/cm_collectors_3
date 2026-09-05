import { flushPromises, mount, shallowMount } from '@vue/test-utils';
import { defineComponent } from 'vue';
import { describe, expect, it, vi } from 'vitest';
import ResourceForm from '../resourceFormDrawer.vue';
import SelectPerformer from '../../com/form/selectPerformer.vue';
import PerformerForm from '../../performer/performerFormDrawer.vue';
import DrawerForm from '../../com/dialog/drawer-form.vue';
import { performerServer } from '@/server/performer.server';
import ElementPlus from 'element-plus';

vi.mock('@/server/performer.server', () => ({ performerServer: { infoById: vi.fn() } }));
vi.mock('@/main', () => ({ eventBus: { on: vi.fn(), off: vi.fn(), emit: vi.fn() } }));
vi.mock('@/storeData/app.storeData', () => ({ appStoreData: () => ({
  currentTagClass: [], currentPerformerBasesIds: [], currentConfigApp: { coverPosterData: [{ width: 200, height: 300 }] },
}) }));
vi.mock('@/language/app.lang', () => ({ AppLang: () => ({ performer: () => '演员', director: () => '导演', btn: (key: string) => key }) }));
vi.mock('@/common/photo', () => ({ getResourceCoverPoster: () => '', getPerformerPhoto: () => '' }));

const openActor = vi.fn();
const actorStub = defineComponent({
  setup(_, { expose }) { expose({ open: openActor, close: vi.fn() }); },
  template: '<div />',
});
const mountResource = () => shallowMount(ResourceForm, {
  global: { plugins: [ElementPlus], renderStubDefaultSlot: true, stubs: {
    performerFormDrawer: actorStub,
    drawerForm: defineComponent({ props: ['modelValue'], template: '<div><slot /></div>' }),
  } },
});

describe('资源中编辑演员的状态隔离', () => {
  it.each(['header', 'footer', 'program'])('真实抽屉通过 %s 关闭会传递一次关闭事件', async (method) => {
    const wrapper = mount(DrawerForm, { props: { modelValue: {} }, global: { plugins: [ElementPlus], stubs: { transition: false } } });
    try {
      wrapper.vm.open();
      await flushPromises();
      if (method === 'program') wrapper.vm.close();
      else if (method === 'header') (document.querySelector('.el-drawer__close-btn') as HTMLElement).click();
      else (Array.from(document.querySelectorAll('.el-drawer button')).find(button => button.textContent?.trim() === 'close') as HTMLElement).click();
      await flushPromises();
      await vi.waitFor(() => expect(wrapper.emitted('closed')).toHaveLength(1));
    } finally { wrapper.unmount(); }
  });
  it('详情返回前关闭资源编辑，不能再弹出演员抽屉', async () => {
    openActor.mockClear();
    let resolveRequest!: (value: Awaited<ReturnType<typeof performerServer.infoById>>) => void;
    vi.mocked(performerServer.infoById).mockImplementation(() => new Promise(resolve => { resolveRequest = resolve; }));
    const wrapper = mountResource();
    try {
      wrapper.findAllComponents(SelectPerformer)[1].vm.$emit('edit', 'actor-1');
      wrapper.findComponent(DrawerForm).vm.$emit('closed');
      resolveRequest({ status: true, statusCode: 200, msg: '', data: { id: 'actor-1' } as never });
      await flushPromises();
      expect(openActor).not.toHaveBeenCalled();
    } finally { wrapper.unmount(); }
  });

  it('演员保存不重置未保存的资源标题和演员关联', async () => {
    const wrapper = mountResource();
    try {
      const drawer = wrapper.findComponent(DrawerForm);
      drawer.props('modelValue').title = '尚未保存的标题';
      const selector = wrapper.findAllComponents(SelectPerformer)[1];
      selector.vm.$emit('update:modelValue', ['actor-1']);
      await flushPromises();
      wrapper.findComponent(PerformerForm).vm.$emit('success', false, { id: 'actor-1', name: '新姓名' });
      await flushPromises();
      expect(drawer.props('modelValue').title).toBe('尚未保存的标题');
      expect(selector.props('modelValue')).toEqual(['actor-1']);
      expect(selector.props('selectedData')).toEqual([{ id: 'actor-1', name: '新姓名' }]);
    } finally { wrapper.unmount(); }
  });
});
