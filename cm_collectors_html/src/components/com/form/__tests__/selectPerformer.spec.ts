import { flushPromises, mount } from '@vue/test-utils';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import SelectPerformer from '../selectPerformer.vue';
import { performerServer } from '@/server/performer.server';
import ElementPlus from 'element-plus';

vi.mock('@/server/performer.server', () => ({
  performerServer: { basicList: vi.fn() },
}));

const actor = { id: 'actor-1', name: '原姓名', aliasName: '别名', keyWords: 'yxm' };
const mountSelector = (props = {}) => mount(SelectPerformer, {
  props: { modelValue: [actor.id], multiple: true, ...props },
  global: {
    stubs: {
      ElSelectV2: {
        props: ['options', 'modelValue'],
        template: '<div><template v-for="item in options.filter(item => modelValue.includes(item.id))" :key="item.id"><slot name="label" :label="item.name" :value="item.id">{{ item.name }}</slot></template></div>',
      },
      ElIcon: { template: '<i><slot /></i>' },
      ElButton: { template: '<button><slot /></button>' },
    },
  },
});

describe('资源中的演员编辑入口', () => {
  beforeEach(() => {
    vi.mocked(performerServer.basicList).mockResolvedValue({ status: true, statusCode: 200, msg: '', data: [actor] });
  });

  it('真实选择框支持右键编辑、改名刷新和移除后候选名称一致', async () => {
    const wrapper = mount(SelectPerformer, {
      attachTo: document.body,
      props: { modelValue: [actor.id], multiple: true, editable: true },
      global: { plugins: [ElementPlus] },
    });
    try {
      await flushPromises();
      await wrapper.get('[title="右键编辑演员"]').trigger('contextmenu', { clientX: 100, clientY: 100 });
      (document.querySelector('.context-menu-item') as HTMLElement).click();
      expect(wrapper.emitted('edit')).toEqual([[actor.id]]);
      expect(wrapper.emitted('update:modelValue')).toBeUndefined();
      await wrapper.setProps({ selectedData: [{ ...actor, name: '新姓名' }] });
      await flushPromises();
      expect(wrapper.get('[title="右键编辑演员"]').text()).toBe('新姓名');
      await wrapper.get('.el-tag__close').trigger('click');
      expect(wrapper.emitted('update:modelValue')).toEqual([[[]]]);
      await wrapper.setProps({ modelValue: [], selectedData: [] });
      expect(wrapper.vm.getOptionsData()[0].name).toBe('新姓名');
    } finally {
      wrapper.unmount();
    }
  });

  it('右键已选姓名打开现有菜单，点击编辑按 ID 发出事件，不修改关联', async () => {
    const wrapper = mountSelector();
    await flushPromises();
    expect(wrapper.find('[title="右键编辑演员"]').exists()).toBe(false);
    await wrapper.setProps({ editable: true });
    expect(wrapper.text()).toContain(actor.name);
    expect(document.querySelector('.context-menu')).toBeNull();
    await wrapper.get('[title="右键编辑演员"]').trigger('contextmenu', { clientX: 100, clientY: 100 });
    expect(document.querySelector('.context-menu')?.textContent).toContain('演员编辑');
    (document.querySelector('.context-menu-item') as HTMLElement).click();
    expect(wrapper.emitted('edit')).toEqual([[actor.id]]);
    expect(wrapper.emitted('update:modelValue')).toBeUndefined();
    wrapper.unmount();
  });

  it('多个演员右键分别指向对应 ID，加载时禁止重复编辑', async () => {
    const secondActor = { ...actor, id: 'actor-2', name: '第二位演员' };
    const wrapper = mountSelector({ editable: true, modelValue: [actor.id, secondActor.id], selectedData: [actor, secondActor] });
    await flushPromises();
    const names = wrapper.findAll('[title="右键编辑演员"]');
    await names[1].trigger('contextmenu');
    expect(document.querySelector('.context-menu__title')?.textContent).toContain(secondActor.name);
    (document.querySelector('.context-menu-item') as HTMLElement).click();
    expect(wrapper.emitted('edit')).toEqual([[secondActor.id]]);
    await wrapper.setProps({ editLoading: true });
    await names[0].trigger('contextmenu');
    (document.querySelector('.context-menu-item') as HTMLElement).click();
    expect(wrapper.emitted('edit')).toHaveLength(1);
    wrapper.unmount();
  });

  it('保存改名后同步入口与选择框标签，保留已选 ID', async () => {
    const wrapper = mountSelector({ editable: true });
    await flushPromises();
    await wrapper.setProps({ selectedData: [{ ...actor, name: '新姓名' }] });
    expect(wrapper.text()).toContain('新姓名');
    expect(wrapper.emitted('update:modelValue')).toBeUndefined();
    expect(wrapper.vm.getOptionsData()[0].name).toBe('新姓名');
    wrapper.unmount();
  });

  it('演员不在职业筛选候选中时，仍保留关联演员的名称和编辑入口', async () => {
    vi.mocked(performerServer.basicList).mockResolvedValue({ status: true, statusCode: 200, msg: '', data: [] });
    const wrapper = mountSelector({ editable: true, selectedData: [actor] });
    await flushPromises();
    expect(wrapper.text()).toContain(actor.name);
    expect(wrapper.vm.getOptionsData()).toEqual([actor]);
    expect(wrapper.emitted('update:modelValue')).toBeUndefined();
    wrapper.unmount();
  });
});
