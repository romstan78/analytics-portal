import { fetchWithAuth, parseJSONResponse } from './promo';

const API_BASE = import.meta.env.VITE_API_BASE || 'http://localhost:8080';

export interface SKUReference {
  id: number; sku: string; brand: string; brand_as: string; created_at: string;
}
export interface NetworkReference {
  id: number; network_name: string; kam: string; network_type: string; top20_segment: string; key_region: string;
}
export interface KAMNetworkReference {
  id: number; kam: string; network_name: string; valid_from: string; created_at: string;
}
export interface MechanicReference {
  id: number; mechanics: string; channel: string; short_code: string; created_at: string;
}
export interface DictionaryData {
  skus: SKUReference[];
  networks: NetworkReference[];
  kam_networks: KAMNetworkReference[];
  mechanics: MechanicReference[];
}

export type DictionaryKind = 'skus' | 'networks' | 'kam-networks' | 'mechanics';
export type DictionaryRow = SKUReference | NetworkReference | KAMNetworkReference | MechanicReference;

async function save<T extends DictionaryRow>(kind: DictionaryKind, data: Partial<T>): Promise<T> {
  const id = data.id;
  const response = await fetchWithAuth(
    `${API_BASE}/api/admin/dictionaries/${kind}${id ? `/${id}` : ''}`,
    { method: id ? 'PATCH' : 'POST', body: JSON.stringify(data) },
  );
  return parseJSONResponse<T>(response, 'Не удалось сохранить запись');
}

export const dictionariesAPI = {
  getAll: (): Promise<DictionaryData> =>
    fetchWithAuth(`${API_BASE}/api/admin/dictionaries`)
      .then(response => parseJSONResponse<DictionaryData>(response, 'Не удалось загрузить справочники')),
  save,
};
