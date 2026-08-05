import request, { requestBlob } from '@/assets/request';
import type {
  I_videoEditPlan,
  I_videoTranscodeAddResult,
  I_videoTranscodeConfig,
  I_videoTranscodeEditPlanResult,
  I_videoTranscodeCapabilities,
  I_videoTranscodeQueueStatus,
  I_videoTranscodeResetResult,
  I_videoTranscodeStartResult,
  I_videoTranscodeTask,
} from '@/dataType/videoTranscode.dataType';

const routerGroupUri = '/videoTranscode';

export const videoTranscodeServer = {
  list: async () => request<I_videoTranscodeTask[]>({
    url: `${routerGroupUri}/list`,
    method: 'get',
  }),
  capabilities: async () => request<I_videoTranscodeCapabilities>({
    url: `${routerGroupUri}/capabilities`,
    method: 'get',
  }),
  add: async (data: {
    resourceIds?: string[];
    dramaSeriesIds?: string[];
    config?: I_videoTranscodeConfig;
  }) => request<I_videoTranscodeAddResult>({
    url: `${routerGroupUri}/add`,
    method: 'post',
    data,
  }),
  updateConfig: async (ids: string[], config: I_videoTranscodeConfig) => request<boolean>({
    url: `${routerGroupUri}/config`,
    method: 'put',
    data: { ids, config },
  }),
  saveEditPlan: async (
    id: string,
    plan: I_videoEditPlan,
    outputMode: 'replace' | 'new_file',
    outputFileName: string,
  ) => request<I_videoTranscodeEditPlanResult>({
    url: `${routerGroupUri}/editPlan/${id}`,
    method: 'put',
    data: { plan, outputMode, outputFileName },
  }),
  thumbnail: async (id: string, at: number) => requestBlob({
    url: `${routerGroupUri}/thumbnail/${id}`,
    method: 'get',
    params: { at },
  }),
  start: async (ids: string[] = [], enableGpu = false) => request<I_videoTranscodeStartResult>({
    url: `${routerGroupUri}/start`,
    method: 'post',
    data: { ids, enableGpu },
  }),
  resetBatch: async (ids: string[]) => request<I_videoTranscodeResetResult>({
    url: `${routerGroupUri}/resetBatch`,
    method: 'post',
    data: { ids },
  }),
  retryReplacement: async (id: string) => request<boolean>({
    url: `${routerGroupUri}/retryReplacement/${id}`,
    method: 'post',
  }),
  saveVerifiedOutputAsNewFile: async (id: string) => request<boolean>({
    url: `${routerGroupUri}/saveVerifiedOutputAsNewFile/${id}`,
    method: 'post',
  }),
  pause: async () => request<boolean>({ url: `${routerGroupUri}/pause`, method: 'post' }),
  resume: async () => request<boolean>({ url: `${routerGroupUri}/resume`, method: 'post' }),
  status: async () => request<I_videoTranscodeQueueStatus>({
    url: `${routerGroupUri}/status`,
    method: 'get',
  }),
  cancel: async (id: string) => request<boolean>({
    url: `${routerGroupUri}/cancel/${id}`,
    method: 'post',
  }),
  delete: async (id: string) => request<boolean>({
    url: `${routerGroupUri}/delete/${id}`,
    method: 'delete',
  }),
  deleteBatch: async (ids: string[]) => request<number>({
    url: `${routerGroupUri}/deleteBatch`,
    method: 'post',
    data: { ids },
  }),
};
