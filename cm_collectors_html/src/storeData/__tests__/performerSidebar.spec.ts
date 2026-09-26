import { beforeEach, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { appStoreData } from '../app.storeData'

const api = vi.hoisted(() => ({ info: vi.fn(), actors: vi.fn(), tags: vi.fn() }))
vi.mock('@/server/filesBases.server', () => ({ filesBasesServer: { infoById: api.info } }))
vi.mock('@/server/performer.server', () => ({ performerServer: { listTopPreferredPerformers: api.actors } }))
vi.mock('@/server/tag.server', () => ({ tagServer: { tagDataByFilesBasesId: api.tags } }))
vi.mock('@/server/app.server', () => ({ appDataServer: {} }))

const library = (id: string, config: object) => ({
  status: true,
  data: {
    id,
    filesRelatedPerformerBases: [{ main: true, performerBases_id: `${id}-actors` }],
    filesBasesSetting: { config_json_data: JSON.stringify(config) },
  },
})

beforeEach(() => {
  vi.resetAllMocks()
  setActivePinia(createPinia())
  api.tags.mockResolvedValue({ status: true, data: { tag: [], tagClass: [] } })
  api.actors.mockResolvedValue({ status: true, data: [] })
})

it('旧配置保留手动演员和照片过滤，并使用默认排序、30天窗口', async () => {
  api.info.mockResolvedValue(library('old', { performerPreferred: ['favorite'], shieldNoPerformerPhoto: true, performerShowNum: 20 }))
  const store = appStoreData()
  await store.initCurrentFilesBases('old')
  expect(api.actors).toHaveBeenCalledWith(['favorite'], 'old-actors', true, 20, 'old', 'default', 30)
  expect(store.currentConfigApp.performerPreferred).toEqual(['favorite'])
  expect(store.currentConfigApp.performerPreferredEnabled).toBe(true)
})

it.each(['default', 'resourceCountDesc', 'hotDesc', 'recentDesc'])('自定义开关关闭后保留选择，重新启用后恢复优先：%s', async (mode) => {
  const config = { performerPreferred: ['favorite'], performerSortMode: mode, shieldNoPerformerPhoto: true }
  api.info.mockResolvedValueOnce(library('A', { ...config, performerPreferredEnabled: false }))
    .mockResolvedValueOnce(library('A', { ...config, performerPreferredEnabled: true }))
  const store = appStoreData()
  await store.initCurrentFilesBases('A')
  expect(api.actors).toHaveBeenNthCalledWith(1, [], 'A-actors', true, 12, 'A', mode, 30)
  expect(store.currentConfigApp.performerPreferred).toEqual(['favorite'])
  await store.initCurrentFilesBases('A')
  expect(api.actors).toHaveBeenNthCalledWith(2, ['favorite'], 'A-actors', true, 12, 'A', mode, 30)
})

it('切换文件库使用各自的排序、统计窗口和照片设置，并重新请求排名', async () => {
  api.info.mockResolvedValueOnce(library('A', { performerSortMode: 'recentDesc', performerRecentDays: 7, shieldNoPerformerPhoto: true }))
    .mockResolvedValueOnce(library('B', { performerSortMode: 'hotDesc', shieldNoPerformerPhoto: false }))
    .mockResolvedValueOnce(library('A', { performerSortMode: 'recentDesc', performerRecentDays: 7, shieldNoPerformerPhoto: true }))
  const store = appStoreData()
  await store.initCurrentFilesBases('A')
  await store.initCurrentFilesBases('B')
  await store.initCurrentFilesBases('A')
  expect(api.actors).toHaveBeenNthCalledWith(1, [], 'A-actors', true, 12, 'A', 'recentDesc', 7)
  expect(api.actors).toHaveBeenNthCalledWith(2, [], 'B-actors', false, 12, 'B', 'hotDesc', 30)
  expect(api.actors).toHaveBeenNthCalledWith(3, [], 'A-actors', true, 12, 'A', 'recentDesc', 7)
})
