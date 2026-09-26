import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { VNode } from 'vue'
import SharedConfigBar from '../SharedConfigBar.vue'
import { sharedConfigServer } from '@/server/sharedConfig.server'
import { filesBasesServer } from '@/server/filesBases.server'
import { ElMessageBox } from 'element-plus'

vi.mock('@/server/sharedConfig.server', () => ({
  sharedConfigServer: { status: vi.fn(), follow: vi.fn(), save: vi.fn() },
}))
vi.mock('@/server/filesBases.server', () => ({ filesBasesServer: { getConfigById: vi.fn() } }))
vi.mock('@/storeData/app.storeData', () => ({
  appStoreData: () => ({ currentFilesBases: { id: 'other' } }),
}))
vi.mock('element-plus', () => ({
  ElMessage: { error: vi.fn(), success: vi.fn() },
  ElMessageBox: { confirm: vi.fn() },
}))
vi.mock('../sharedConfig/SharedDisplayFields.vue', () => ({
  default: {
    props: ['config'],
    template: '<input data-test="public-value" v-model.number="config.pageLimit" />',
  },
}))
vi.mock('../sharedConfig/SharedImportFields.vue', () => ({
  default: {
    props: ['config'],
    template: '<input data-test="public-value" v-model.number="config.timeout" />',
  },
}))
vi.mock('../sharedConfig/SharedScraperFields.vue', () => ({
  default: {
    props: ['config'],
    template: '<input data-test="public-value" v-model.number="config.timeout" />',
  },
}))

const ok = <T>(data: T) => ({ status: true, statusCode: 200, msg: '', data })
const state = (following = true) => ({
  module: 'display' as const,
  following,
  canRestore: true,
  available: true,
  revision: 3,
  fields: ['pageLimit'],
  config: { pageLimit: 64 },
  libraries: [{ id: 'A', name: '电影库' }],
})
const makeWrapper = () =>
  mount(SharedConfigBar, {
    props: {
      filesBasesId: 'A',
      module: 'display',
      config: { pageLimit: 32, coverDisplayTag: ['unsaved-local'], sampleFolder: 'unsaved-dir' },
      localHint: '标签独立',
    },
    global: {
      directives: { loading: {} },
      stubs: {
        ElButton: { template: '<button><slot /></button>' },
        ElSwitch: {
          props: ['modelValue'],
          emits: ['change'],
          template:
            '<input type="checkbox" :checked="modelValue" @change="$emit(\'change\', !modelValue)" />',
        },
        ElDialog: {
          props: ['modelValue', 'title'],
          template:
            '<section v-if="modelValue" role="dialog"><h2>{{ title }}</h2><slot /><slot name="footer" /></section>',
        },
        ElForm: { template: '<div><slot /></div>' },
        ElTag: { template: '<span><slot /></span>' },
        ElAlert: true,
      },
    },
  })
const click = async (w: ReturnType<typeof makeWrapper>, label: string) => {
  await w
    .findAll('button')
    .find((b) => b.text() === label)!
    .trigger('click')
  await flushPromises()
}

describe('公共配置独立弹窗', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    vi.mocked(sharedConfigServer.status).mockResolvedValue(ok(state()))
    vi.mocked(sharedConfigServer.follow).mockResolvedValue(ok(true))
    vi.mocked(sharedConfigServer.save).mockResolvedValue(ok(true))
    vi.mocked(filesBasesServer.getConfigById).mockResolvedValue(
      ok('{"pageLimit":64,"coverDisplayTag":["local"]}'),
    )
    vi.mocked(ElMessageBox.confirm).mockResolvedValue(
      'confirm' as Awaited<ReturnType<typeof ElMessageBox.confirm>>,
    )
  })
  it('独立显示公共值及影响范围，取消丢弃弹窗草稿且不触碰本库', async () => {
    const w = makeWrapper()
    await flushPromises()
    expect(w.find('[role="dialog"]').exists()).toBe(false)
    await click(w, '打开公共配置')
    expect(w.get('[role="dialog"]').text()).toContain('公共配置 · 基础展示')
    expect(w.get('[role="dialog"]').text()).toContain('电影库')
    expect((w.get('[data-test="public-value"]').element as HTMLInputElement).value).toBe('64')
    await w.get('[data-test="public-value"]').setValue('99')
    await click(w, '取消')
    expect(w.vm.dialogOpen).toBe(false)
    expect(w.emitted('config')).toBeUndefined()
    expect(w.props('config')).toMatchObject({ pageLimit: 32, sampleFolder: 'unsaved-dir' })
    expect(sharedConfigServer.save).not.toHaveBeenCalled()
    await click(w, '打开公共配置')
    expect((w.get('[data-test="public-value"]').element as HTMLInputElement).value).toBe('64')
    w.unmount()
  })
  it('只提交公共字段，保存后保留本库未保存的独立字段', async () => {
    const w = makeWrapper()
    await flushPromises()
    await click(w, '打开公共配置')
    await w.get('[data-test="public-value"]').setValue('99')
    vi.mocked(sharedConfigServer.save).mockImplementation(async () => {
      vi.mocked(sharedConfigServer.status).mockResolvedValue(
        ok({ ...state(), revision: 4, config: { pageLimit: 99 } }),
      )
      return ok(true)
    })
    await click(w, '保存公共配置')
    expect(sharedConfigServer.save).toHaveBeenCalledWith('display', 3, { pageLimit: 99 })
    expect(w.emitted('config')?.at(-1)).toEqual([
      { pageLimit: 99, coverDisplayTag: ['unsaved-local'], sampleFolder: 'unsaved-dir' },
    ])
    expect(w.vm.dialogOpen).toBe(false)
    expect(w.emitted('saved')).toHaveLength(1)
    w.unmount()
  })
  it('首次创建取本库公共字段，取消不创建，保存不自动跟随', async () => {
    vi.mocked(sharedConfigServer.status).mockResolvedValue(
      ok({ ...state(false), available: false, revision: 0, libraries: [] }),
    )
    const w = makeWrapper()
    await flushPromises()
    await click(w, '创建公共配置…')
    expect((w.get('[data-test="public-value"]').element as HTMLInputElement).value).toBe('32')
    expect(sharedConfigServer.save).not.toHaveBeenCalled()
    await click(w, '取消')
    await click(w, '创建公共配置…')
    await click(w, '保存公共配置')
    expect(sharedConfigServer.save).toHaveBeenCalledWith('display', 0, { pageLimit: 32 })
    expect(sharedConfigServer.follow).not.toHaveBeenCalled()
    expect(w.emitted('config')).toBeUndefined()
    w.unmount()
  })
  it.each(['刷新发现冲突', '服务端拒绝'] as const)(
    '%s 时保留编辑内容，不报告成功',
    async (mode) => {
      const w = makeWrapper()
      await flushPromises()
      await click(w, '打开公共配置')
      await w.get('[data-test="public-value"]').setValue('99')
      if (mode === '刷新发现冲突')
        vi.mocked(sharedConfigServer.status).mockResolvedValue(ok({ ...state(), revision: 4 }))
      else
        vi.mocked(sharedConfigServer.save).mockResolvedValue({
          status: false,
          statusCode: 500,
          msg: '版本冲突',
          data: false,
        })
      await click(w, '保存公共配置')
      expect(w.vm.dialogOpen).toBe(true)
      expect((w.get('[data-test="public-value"]').element as HTMLInputElement).value).toBe('99')
      expect(w.emitted('saved')).toBeUndefined()
      expect(w.emitted('config')).toBeUndefined()
      if (mode === '刷新发现冲突') expect(sharedConfigServer.save).not.toHaveBeenCalled()
      w.unmount()
    },
  )
  it('关闭跟随带版本号，取消切换不写入', async () => {
    const w = makeWrapper()
    await flushPromises()
    vi.mocked(ElMessageBox.confirm).mockRejectedValueOnce('cancel')
    await w.get('input[type="checkbox"]').trigger('change')
    await flushPromises()
    expect(sharedConfigServer.follow).not.toHaveBeenCalled()
    await w.get('input[type="checkbox"]').trigger('change')
    await flushPromises()
    expect(sharedConfigServer.follow).toHaveBeenCalledWith('A', 'display', false, 3, 'keep')
    w.unmount()
  })
  it('选择恢复原配置后提交 restore，缺失参数恢复默认值而不是残留公共值', async () => {
    vi.mocked(filesBasesServer.getConfigById).mockResolvedValue(ok('{"sampleFolder":"saved-dir"}'))
    vi.mocked(ElMessageBox.confirm).mockImplementationOnce(async (message) => {
      const choices = mount({ render: () => message as VNode })
      expect(choices.text()).toContain('恢复跟随前的本库配置')
      const restore = choices.findAll('input')[1]
      expect(restore.attributes('disabled')).toBeUndefined()
      await restore.setValue()
      choices.unmount()
      return 'confirm' as Awaited<ReturnType<typeof ElMessageBox.confirm>>
    })
    const w = makeWrapper()
    await flushPromises()
    await w.setProps({ config: { pageLimit: 999, sampleFolder: 'unsaved-dir' } })
    await w.get('input[type="checkbox"]').trigger('change')
    await flushPromises()
    expect(sharedConfigServer.follow).toHaveBeenCalledWith('A', 'display', false, 3, 'restore')
    expect(w.emitted('config')?.at(-1)).toEqual([expect.objectContaining({ pageLimit: 32, sampleFolder: 'saved-dir' })])
    w.unmount()
  })
  it('没有历史快照时禁用恢复并提示，只保留当前公共配置', async () => {
    vi.mocked(sharedConfigServer.status).mockResolvedValue(ok({ ...state(), canRestore: false }))
    vi.mocked(ElMessageBox.confirm).mockImplementationOnce(async (message) => {
      const choices = mount({ render: () => message as VNode })
      expect(choices.text()).toContain('没有历史快照')
      expect(choices.findAll('input')[1].attributes('disabled')).toBeDefined()
      choices.unmount()
      return 'confirm' as Awaited<ReturnType<typeof ElMessageBox.confirm>>
    })
    const w = makeWrapper()
    await flushPromises()
    await w.get('input[type="checkbox"]').trigger('change')
    await flushPromises()
    expect(sharedConfigServer.follow).toHaveBeenCalledWith('A', 'display', false, 3, 'keep')
    w.unmount()
  })
  it.each(['import', 'scraper'] as const)('%s 跟随切换读取对应配置分组', async (module) => {
    vi.mocked(sharedConfigServer.status).mockResolvedValue(
      ok({ ...state(), module, fields: ['timeout'], config: { timeout: 30 } }),
    )
    vi.mocked(filesBasesServer.getConfigById).mockResolvedValue(
      ok('{"timeout":30,"scanDiskPaths":["local-dir"]}'),
    )
    const w = makeWrapper()
    await w.setProps({ module, config: { timeout: 60, scanDiskPaths: ['local-dir'] } })
    await flushPromises()
    await w.get('input[type="checkbox"]').trigger('change')
    await flushPromises()
    expect(sharedConfigServer.follow).toHaveBeenCalledWith('A', module, false, 3, 'keep')
    expect(filesBasesServer.getConfigById).toHaveBeenCalledWith(
      'A',
      module === 'import' ? 'importScanDisk' : 'scraper',
    )
    expect(w.emitted('config')?.at(-1)).toEqual([expect.objectContaining({ timeout: 30, scanDiskPaths: ['local-dir'] })])
    w.unmount()
  })
  it('切换文件库关闭旧弹窗，不将旧草稿带入新库', async () => {
    const w = makeWrapper()
    await flushPromises()
    await click(w, '打开公共配置')
    await w.get('[data-test="public-value"]').setValue('99')
    await w.setProps({ filesBasesId: 'B' })
    await flushPromises()
    expect(w.vm.dialogOpen).toBe(false)
    expect(w.emitted('config')).toBeUndefined()
    expect(sharedConfigServer.save).not.toHaveBeenCalled()
    w.unmount()
  })
})
