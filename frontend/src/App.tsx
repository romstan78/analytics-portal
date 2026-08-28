import { lazy, Suspense, useEffect, useState } from 'react';
import { Routes, Route, useLocation, useNavigate } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { ThemeProvider, CssBaseline, Box, Typography, Button, CircularProgress } from '@mui/material';
import { ArrowBack as ArrowBackIcon } from '@mui/icons-material';
import { modernTheme } from './theme';
import ErrorBoundary from './components/ErrorBoundary';
import { getToken, logout } from './api/auth';
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
  const [auth, setAuth] = useState<AuthState>(() => ({
    token: getToken(),
    username: localStorage.getItem('username'),
    role: localStorage.getItem('role'),
  }));

  const handleLogin = (data: SessionData) => {
    setAuth({
      token: data.token ?? null,
      username: data.username ?? null,
      role: data.role ?? null,
    });
  };

  const handleLogout = () => {
    logout();
    setAuth({ token: null, username: null, role: null });
  };

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
            </Routes>
          </Suspense>
        </ErrorBoundary>
      </Box>
    </ThemeProvider>
  );
}
