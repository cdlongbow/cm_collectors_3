import { mount, flushPromises } from '@vue/test-utils';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import SharedConfigBar from '../SharedConfigBar.vue';
import { sharedConfigServer } from '@/server/sharedConfig.server';
import { filesBasesServer } from '@/server/filesBases.server';
import { ElMessageBox } from 'element-plus';

vi.mock('@/server/sharedConfig.server', () => ({ sharedConfigServer: { status: vi.fn(), follow: vi.fn(), save: vi.fn() } }));
vi.mock('@/server/filesBases.server', () => ({ filesBasesServer: { getConfigById: vi.fn() } }));
vi.mock('@/storeData/app.storeData', () => ({ appStoreData: () => ({ currentFilesBases: { id: 'other' } }) }));
vi.mock('element-plus', () => ({ ElMessage: { error: vi.fn(), success: vi.fn() }, ElMessageBox: { confirm: vi.fn() } }));

const ok = <T,>(data: T) => ({ status: true, statusCode: 200, msg: '', data });
const state = (following = true) => ({ module: 'display' as const, following, available: true, revision: 3, fields: ['pageLimit'], config: { pageLimit: 64 }, libraries: [{ id: 'A', name: '电影库' }] });
const makeWrapper = () => mount(SharedConfigBar, {
  props: { filesBasesId: 'A', module: 'display', config: { pageLimit: 32, coverDisplayTag: ['local'] }, localHint: '标签独立' },
  global: { directives: { loading: {} }, stubs: {
    ElButton: { template: '<button><slot /></button>' },
    ElSwitch: { props: ['modelValue'], emits: ['change'], template: '<input type="checkbox" :checked="modelValue" @change="$emit(\'change\', !modelValue)" />' },
  } },
});

describe('公共配置交互边界', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(sharedConfigServer.status).mockResolvedValue(ok(state()));
    vi.mocked(sharedConfigServer.follow).mockResolvedValue(ok(true));
    vi.mocked(sharedConfigServer.save).mockResolvedValue(ok(true));
    vi.mocked(filesBasesServer.getConfigById).mockResolvedValue(ok('{"pageLimit":64,"coverDisplayTag":["local"]}'));
    vi.mocked(ElMessageBox.confirm).mockResolvedValue('confirm');
  });
  it('跟随只锁定共享字段，编辑公共配置时只开放共享字段，取消恢复草稿', async () => {
    const w = makeWrapper(); await flushPromises();
    expect(w.vm.fieldDisabled('pageLimit')).toBe(true);
    expect(w.vm.fieldDisabled('coverDisplayTag')).toBe(false);
    await w.get('button').trigger('click'); await flushPromises();
    expect(w.vm.editing).toBe(true);
    expect(w.vm.fieldDisabled('pageLimit')).toBe(false);
    expect(w.vm.fieldDisabled('coverDisplayTag')).toBe(true);
    expect(w.text()).toContain('电影库');
    expect(w.emitted('config')?.[0]).toEqual([{ pageLimit: 64, coverDisplayTag: ['local'] }]);
    await w.findAll('button').find(b => b.text() === '取消编辑')!.trigger('click');
    expect(w.emitted('config')?.at(-1)).toEqual([{ pageLimit: 32, coverDisplayTag: ['local'] }]);
    expect(sharedConfigServer.save).not.toHaveBeenCalled();
    w.unmount();
  });
  it('关闭跟随带版本号并加载物化后的配置，取消切换不写入', async () => {
    const w = makeWrapper(); await flushPromises();
    vi.mocked(ElMessageBox.confirm).mockRejectedValueOnce('cancel');
    await w.get('input').trigger('change'); await flushPromises();
    expect(sharedConfigServer.follow).not.toHaveBeenCalled();
    await w.get('input').trigger('change'); await flushPromises();
    expect(sharedConfigServer.follow).toHaveBeenCalledWith('A', 'display', false, 3);
    expect(w.emitted('config')?.at(-1)).toEqual([{ pageLimit: 64, coverDisplayTag: ['local'] }]);
    w.unmount();
  });
  it('公共保存版本冲突时保留编辑内容，不报告成功', async () => {
    const w = makeWrapper(); await flushPromises();
    await w.get('button').trigger('click'); await flushPromises();
    vi.mocked(sharedConfigServer.save).mockResolvedValue({ status: false, statusCode: 500, msg: '版本冲突', data: false });
    await w.findAll('button').find(b => b.text() === '保存公共配置')!.trigger('click'); await flushPromises();
    expect(w.vm.editing).toBe(true);
    expect(w.emitted('saved')).toBeUndefined();
    w.unmount();
  });
});
