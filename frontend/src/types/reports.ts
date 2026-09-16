// Типы выгрузки отчётов PDF/PPTX.
//
// Контракт описан в Go (backend/models/report.go) и собирается генератором в
// ./api.generated.ts. Здесь — только сужения значений для интерфейса.

import type { ReportRequest } from './api.generated';

export type {
  ReportBlock,
  ReportRequest,
  ReportJobStatus,
  ReportCreateResponse,
} from './api.generated';

export type ReportFormat = 'pdf' | 'pptx';
export type ReportUnit = 'rub' | 'units';
export type ReportJobState = 'queued' | 'running' | 'ready' | 'failed';

export const REPORT_TABLE_LIMITS = [10, 20] as const;

// Черновик конструктора: то же тело запроса с суженными значениями.
export type ReportDraft = Omit<ReportRequest, 'unit' | 'formats'> & {
  unit: ReportUnit;
  formats: ReportFormat[];
};
