import { lazy, Suspense, useEffect, useRef, useState } from 'react';
import { Routes, Route, Navigate, useLocation, useNavigate } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { ThemeProvider, CssBaseline, Box, Typography, Button, CircularProgress } from '@mui/material';
import { ArrowBack as ArrowBackIcon } from '@mui/icons-material';
import { modernTheme } from './theme';
import ErrorBoundary from './components/ErrorBoundary';
import { getToken, logout } from './api/auth';
import { forgetReturnPath, takeReturnPath } from './utils/returnPath';
import type { SessionData } from './api/auth';

const Login = lazy(() => import('./pages/Login'));
const Home = lazy(() => import('./pages/Home'));
const InternetSales = lazy(() => import('./pages/InternetSales'));
const PromoAnalysis = lazy(() => import('./pages/PromoAnalysis'));
const NetworkRegistry = lazy(() => import('./pages/NetworkRegistry'));

function PageLoader() {
  return (
    <Box sx={{ display: 'grid', minHeight: '100vh', placeItems: 'center' }}>
      <CircularProgress aria-label="Загрузка страницы" />
    </Box>
  );
}

interface PlaceholderPageProps {
  title: string;
  description: string;
}

function PlaceholderPage({ title, description }: PlaceholderPageProps) {
  const navigate = useNavigate();

  return (
    <Box sx={{ p: 4 }}>
      <Button startIcon={<ArrowBackIcon />} onClick={() => navigate('/')} sx={{ mb: 4 }}>
        На главную
      </Button>
      <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', minHeight: '50vh', textAlign: 'center' }}>
        <Typography variant="h3" gutterBottom sx={{ fontWeight: 700, color: 'text.secondary' }}>{title}</Typography>
        <Typography variant="h6" color="text.secondary" sx={{ mb: 3 }}>{description}</Typography>
        <Typography variant="body1" color="text.disabled">🚧 Раздел находится в разработке</Typography>
      </Box>
    </Box>
  );
}

interface AuthState {
  token: string | null;
  username: string | null;
  role: string | null;
}

export default function App() {
  const queryClient = useQueryClient();
  const location = useLocation();
  const navigate = useNavigate();
  const [auth, setAuth] = useState<AuthState>(() => ({
    token: getToken(),
    username: localStorage.getItem('username'),
    role: localStorage.getItem('role'),
  }));

  // Куда уйти сразу после входа: на прерванный истечением сессии раздел, а
  // иначе на главную. Цель держим в ref и переходим уже после рендера: вызов
  // navigate прямо из обработчика применяется позже, чем монтируются маршруты,
  // и catch-all успевал увести на главную с адреса /login, отменив возврат.
  const pendingReturn = useRef<string | null>(null);

  const handleLogin = (data: SessionData) => {
    pendingReturn.current = takeReturnPath(data.username ?? null) ?? '/';
    setAuth({
      token: data.token ?? null,
      username: data.username ?? null,
      role: data.role ?? null,
    });
  };

  const handleLogout = () => {
    forgetReturnPath();
    logout();
    setAuth({ token: null, username: null, role: null });
  };

  useEffect(() => {
    const target = pendingReturn.current;
    if (target === null) return;
    // Цель забываем сразу: переход одноразовый, дальше пользователь ходит сам.
    pendingReturn.current = null;
    // Адрес заменяем: «назад» не должно возвращать на форму входа.
    navigate(target, { replace: true });
  }, [auth.token, navigate]);

  // Слушаем разлогин: и кнопкой, и принудительный из api/promo.ts — logout()
  // в обоих случаях шлёт это событие.
  //
  // Кэш запросов чистится здесь же. Он живёт в QueryClient вне дерева и смену
  // пользователя переживает, а ключи запросов пользователя не содержат, поэтому
  // без очистки следующий вошедший получает списки предыдущего: админ после
  // КАМа видит его сети, и наоборот.
  useEffect(() => {
    const onForceLogout = () => {
      queryClient.clear();
      setAuth({ token: null, username: null, role: null });
    };
    window.addEventListener('auth:logout', onForceLogout);
    return () => window.removeEventListener('auth:logout', onForceLogout);
  }, [queryClient]);

  if (!auth.token) {
    return (
      <ThemeProvider theme={modernTheme}>
        <CssBaseline />
        {/* Граница снаружи Suspense: так она ловит и падение отрисовки, и
            несостоявшуюся загрузку ленивого чанка. */}
        <ErrorBoundary>
          <Suspense fallback={<PageLoader />}>
            <Login onLogin={handleLogin} />
          </Suspense>
        </ErrorBoundary>
      </ThemeProvider>
    );
  }

  // Пока возврат не состоялся, маршруты не монтируем: иначе адрес /login
  // попал бы в catch-all и увёл бы на главную вместо прерванного раздела.
  if (pendingReturn.current !== null) {
    return (
      <ThemeProvider theme={modernTheme}>
        <CssBaseline />
        <PageLoader />
      </ThemeProvider>
    );
  }

  return (
    <ThemeProvider theme={modernTheme}>
      <CssBaseline />
      <Box sx={{ display: 'flex', flexDirection: 'column', minHeight: '100vh', bgcolor: 'background.default' }}>
        {/* Ключ сброса — путь: уход с упавшего экрана обязан показать новый,
            а не оставить сообщение об ошибке предыдущего. */}
        <ErrorBoundary resetKey={location.pathname}>
          <Suspense fallback={<PageLoader />}>
            <Routes>
              <Route path="/" element={<Home onLogout={handleLogout} />} />
              <Route path="/internet-sales" element={<InternetSales />} />
              <Route path="/promo-analysis" element={<PromoAnalysis role={auth.role} />} />
              <Route path="/sales-analysis" element={<PlaceholderPage title="Анализ продаж" description="Динамика продаж по периодам" />} />
              <Route path="/network-registry" element={<NetworkRegistry role={auth.role} />} />
              <Route path="/turnover" element={<PlaceholderPage title="Оборачиваемость" description="Анализ оборотов запасов" />} />
              <Route path="/like-for-like" element={<PlaceholderPage title="Продажи Like For Like" description="Сравнение продаж LFL" />} />
              {/* Форма входа живёт вне Routes, поэтому адрес /login маршрута не
                  имеет: после истечения сессии fetchWithAuth уводит туда
                  (api/promo.ts), и без этого правила вошедший заново оставался
                  бы на пустом экране. Заодно закрывает старые ссылки. */}
              <Route path="*" element={<Navigate to="/" replace />} />
            </Routes>
          </Suspense>
        </ErrorBoundary>
      </Box>
    </ThemeProvider>
  );
}
