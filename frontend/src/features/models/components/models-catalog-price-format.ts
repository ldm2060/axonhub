// 职责: 保存模型报价编辑的非组件导出（样式、键构造、格式化钩子、默认选择逻辑），
//       保持 models-catalog-prices.tsx 只导出组件以满足 react-refresh 约束。
// 关联文件:
//   frontend/src/features/models/components/models-catalog-prices.tsx
//   frontend/src/features/models/components/models-catalog-import.tsx
//   frontend/src/features/models/components/models-table.tsx
//   frontend/src/features/models/data/catalog-import.ts
// 注意事项:
//   defaultSelection 依赖 data/schema 的 referencePriceFields；两处 import 必须同步。
import { useTranslation } from 'react-i18next';
import type { ImportRow, LocalRate, Offer, PriceSelection, ImportSelection } from '../data/catalog-import';
import { modelTypeSchema, referencePriceFields } from '../data/schema';

export const selectStyle = 'h-9 max-w-full rounded-md border bg-background px-2 text-sm';
export const offerKey = (offer: { provider: string; modelID: string }) => JSON.stringify([offer.provider, offer.modelID]);

export function useReferencePriceFormat() {
  const { t, i18n } = useTranslation();
  return (value: number) =>
    t('currencies.format', { val: value, currency: 'USD', locale: i18n.language, minimumFractionDigits: 0, maximumFractionDigits: 12 });
}

function findOffer(row: ImportRow, source: Pick<Offer, 'provider' | 'modelID'> | null) {
  return row.offers.find((offer) => offer.provider === source?.provider && offer.modelID === source.modelID);
}

export function defaultRequest(modelID: string, route: ImportRow['routes'][number]) {
  if (route.requests.includes(modelID)) return modelID;
  if (route.requests.includes(route.actual)) return route.actual;
  if (!route.requests.length) return '';
  return route.requests.length === 1 ? route.requests[0] : undefined;
}

export function needsRebind(local: LocalRate | undefined, offer: Offer) {
  return local?.follow && (local.provider !== offer.provider || local.modelID !== offer.modelID || local.canonical !== offer.canonical);
}

export function defaultSelection(row: ImportRow, offer = findOffer(row, row.preferred) ?? null): ImportSelection {
  const prices: PriceSelection[] = [];
  for (const field of referencePriceFields) {
    const local = row.prices.find((p) => p.field === field);
    if (row.version && !local?.follow) continue;
    const source = local ? findOffer(row, local) : offer;
    const rate = source?.rates.find((r) => r.field === field);
    if (!source || rate?.base == null || needsRebind(local, source) || local?.value === rate.base) continue;
    prices.push({
      field,
      action: 'FOLLOW',
      expected: rate.base,
      identity: source.identity,
      provider: source.provider,
      modelID: source.modelID,
    });
  }
  return {
    type: offer?.type ? (modelTypeSchema.safeParse(offer.type).data ?? '') : 'chat',
    identity: offer?.identity ?? '',
    modelID: row.modelID,
    members: row.members,
    manualMerge: row.manualMerge,
    routes: row.version
      ? []
      : row.routes.map((route) => ({ channelID: route.channelID, actual: route.actual, request: defaultRequest(row.modelID, route) })),
    version: row.version,
    name: row.version ? row.name : offer?.name || row.modelID,
    developer: row.version ? '' : offer?.developer || '',
    provider: offer?.provider || '',
    sourceModelID: offer?.modelID || '',
    prices,
  };
}
