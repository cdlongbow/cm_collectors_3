import type { I_config_app, T_detailsVisibleField } from '@/dataType/config.dataType';

export const isDetailsFieldVisible = (
  config: Pick<I_config_app, 'detailsVisibleFields'>,
  field: T_detailsVisibleField,
) => {
  const fields = config.detailsVisibleFields;
  // 兼容尚未经过默认配置合并的旧数据；明确保存空数组时则全部隐藏。
  return !Array.isArray(fields) || fields.includes(field);
};
