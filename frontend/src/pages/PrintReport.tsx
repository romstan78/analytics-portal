// Печатная страница отчёта: /print/report/:id?token=…
//
// Её открывает headless Chromium из фонового задания, а не пользователь:
// сессии здесь нет, данные отдаются по одноразовому токену задания, шапки и
// меню нет. Готовность страница сообщает флагом window.__reportReady —
// после загрузки данных, применения шрифта и двух кадров отрисовки, чтобы
// Recharts успел дорисовать SVG; рендер ждёт флага, а не таймера. Ошибка
// уходит в window.__reportError, и рендер падает сразу, а не по тайм-ауту.

import { useEffect, useState } from 'react';
import { useParams, useSearchParams } from 'react-router-dom';
import { Box, Typography } from '@mui/material';
import ReportDocument from '../components/report/ReportDocument';
import { reportAPI } from '../api/reports';
import type { ReportPrint } from '../types/reports';
import '../print.css';

declare global {
  interface Window {
    __reportReady?: boolean;
    __reportError?: string;
  }
}

function errorText(error: unknown): string {
  if (error && typeof error === 'object' && 'message' in error && typeof (error as { message: unknown }).message === 'string') {
    return (error as { message: string }).message;
  }
  return 'Не удалось загрузить данные отчёта';
}

export default function PrintReport() {
  const { id = '' } = useParams<{ id: string }>();
  const [params] = useSearchParams();
  const token = params.get('token') ?? '';
  const [report, setReport] = useState<ReportPrint | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const error = !id || !token ? 'Нет идентификатора отчёта или токена' : loadError;

  useEffect(() => {
    window.__reportReady = false;
    window.__reportError = undefined;
    if (!id || !token) return;
    let cancelled = false;
    reportAPI.getPrint(id, token)
      .then((data) => { if (!cancelled) setReport(data); })
      .catch((err: unknown) => { if (!cancelled) setLoadError(errorText(err)); });
    return () => { cancelled = true; };
  }, [id, token]);

  useEffect(() => {
    if (error) {
      window.__reportError = error;
      return;
    }
    if (!report) return;
    let cancelled = false;
    const fontsReady = typeof document.fonts?.ready?.then === 'function' ? document.fonts.ready : Promise.resolve();
    fontsReady.then(() => {
      requestAnimationFrame(() => requestAnimationFrame(() => {
        if (!cancelled) window.__reportReady = true;
      }));
    });
    return () => { cancelled = true; };
  }, [report, error]);

  if (error) {
    return (
      <Box sx={{ p: 4 }} data-report-error>
        <Typography variant="h6" color="error">{error}</Typography>
      </Box>
    );
  }
  if (!report) {
    return <Box sx={{ p: 4 }}><Typography color="text.secondary">Подготовка отчёта…</Typography></Box>;
  }
  return <ReportDocument report={report} />;
}
