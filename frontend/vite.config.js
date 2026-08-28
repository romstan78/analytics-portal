import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
//
// Прокси на /api здесь нет намеренно: приложение обращается к backend по
// абсолютному адресу из VITE_API_BASE (frontend/src/api/promo.ts), поэтому
// проксировать нечего. Раньше здесь стояла запись на localhost:8080 — она не
// использовалась, но сбивала с толку тех, кто отлаживал демо-контур, где
// backend слушает 8081.
export default defineConfig({
  plugins: [react()],
})
