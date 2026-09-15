import { fetchWithAuth, parseJSONResponse, buildParams } from './promo';
import type { BudgetResponse, BudgetVersion, BudgetSources, BudgetPromoResponse, BudgetParams, BudgetCellInput } from '../types/budget';
const base = `${import.meta.env.VITE_API_BASE || 'http://localhost:8080'}/api/budget`;
const query = (p: object) => buildParams(p as Record<string, string | number | string[]>);
async function send<T>(path: string, method: string, body: unknown): Promise<T> {
  return parseJSONResponse<T>(await fetchWithAuth(`${base}${path}`, { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }), 'Не удалось сохранить бюджет');
}
export const budgetAPI = {
  get: (p: BudgetParams) => fetchWithAuth(`${base}?${query(p)}`).then(r => parseJSONResponse<BudgetResponse>(r, 'Не удалось загрузить бюджет')),
  versions: (year: number) => fetchWithAuth(`${base}/versions?year=${year}`).then(r => parseJSONResponse<BudgetVersion[]>(r, 'Не удалось загрузить версии')),
  create: (year: number, code: string, sources: BudgetSources) => send<BudgetVersion>('/versions', 'POST', { year, code, sources }),
  sources: (id: number, sources: BudgetSources, updated_at: string) => send(`/versions/${id}/sources`, 'PATCH', { sources, updated_at }),
  export: async (p: BudgetParams) => { const response = await fetchWithAuth(`${base}/export?${query(p)}`); if (!response.ok) {
    await parseJSONResponse(response, 'Ошибка выгрузки');
  } return response.blob(); },
  freeze: (id: number, updated_at: string) => send(`/versions/${id}/freeze`, 'POST', { updated_at }),
  cell: (id: number, p: BudgetParams, body: BudgetCellInput, reset = false) => send(`/versions/${id}/cells?${query(p)}`, reset ? 'DELETE' : 'PUT', body),
  promos: (p: BudgetParams, brand: string, quarter: number, page: number, status: string, type: string) => fetchWithAuth(`${base}/promos?${query({ ...p, brand, quarter, page, status, type })}`).then(r => parseJSONResponse<BudgetPromoResponse>(r, 'Не удалось загрузить промо')),
  include: (id: number, promo_ids: number[], included: boolean, updated_at: string) => send(`/versions/${id}/promos`, 'PUT', { promo_ids, included, updated_at }),
};
