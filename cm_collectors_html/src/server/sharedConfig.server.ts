import request from '@/assets/request';

export type SharedModule = 'display';
export interface SharedConfigState {
  module: SharedModule;
  available: boolean;
  following: boolean;
  revision: number;
  fields: string[];
  config: Record<string, unknown>;
  libraries: { id: string; name: string }[];
}

export const sharedConfigServer = {
  status: (id: string, module: SharedModule) => request<SharedConfigState>({ url: `/filesBases/shared/${module}/${id}`, method: 'get' }),
  save: (module: SharedModule, revision: number, config: object) => request<boolean>({
    url: `/filesBases/shared/${module}`, method: 'put', data: { revision, config: JSON.stringify(config) },
  }),
  follow: (id: string, module: SharedModule, following: boolean, revision: number) => request<boolean>({
    url: `/filesBases/follow/${module}/${id}`, method: 'put', data: { following, revision },
  }),
};
