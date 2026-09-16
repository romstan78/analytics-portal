// Клиент выгрузки отчётов PDF/PPTX по витрине реестра сетей.
// Общие хелперы (авторизация, разбор ответа) берём из ./promo.

import { fetchWithAuth, parseJSONResponse } from './promo';
import type { ReportBlock, ReportCreateResponse, ReportJobStatus, ReportRequest } from '../types/reports';

const API_BASE = import.meta.env.VITE_API_BASE || 'http://localhost:8080';

export const reportAPI = {
  // Каталог блоков: порядок ответа — порядок по умолчанию.
  getBlocks: (): Promise<ReportBlock[]> =>
    fetchWithAuth(`${API_BASE}/api/reports/blocks`)
      .then(r => parseJSONResponse<ReportBlock[]>(r, 'Ошибка загрузки каталога блоков')),

  // Запуск: сервер читает витрину один раз и заводит по заданию на формат.
  // 429 — предыдущий отчёт ещё готовится; текст ошибки приходит с сервера.
  create: (request: ReportRequest): Promise<ReportCreateResponse> =>
    fetchWithAuth(`${API_BASE}/api/reports`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(request),
    }, 60000).then(r => parseJSONResponse<ReportCreateResponse>(r, 'Не удалось запустить подготовку отчёта')),

  // Статус задания; 404 означает, что задание истекло или чужое.
  getJob: (id: string): Promise<ReportJobStatus> =>
    fetchWithAuth(`${API_BASE}/api/reports/${encodeURIComponent(id)}`)
      .then(r => parseJSONResponse<ReportJobStatus>(r, 'Не удалось получить состояние отчёта')),

  // Готовый файл. Ответ — blob, поэтому без parseJSONResponse: ошибка
  // приходит JSON-ом только когда файл не отдан.
  download: async (id: string): Promise<Blob> => {
    const response = await fetchWithAuth(`${API_BASE}/api/reports/${encodeURIComponent(id)}/download`, {}, 120000);
    if (!response.ok) {
      const payload = await response.json().catch(() => ({})) as { error?: string };
      throw new Error(payload.error || 'Не удалось скачать отчёт');
    }
    return response.blob();
  },
};
