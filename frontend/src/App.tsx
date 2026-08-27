import { lazy, Suspense, useEffect, useState } from 'react';
import { Routes, Route, useLocation, useNavigate } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { createTheme, ThemeProvider, CssBaseline, Box, Typography, Button, CircularProgress } from '@mui/material';
import { ArrowBack as ArrowBackIcon } from '@mui/icons-material';
// Подключает стили MuiDataGrid к типу темы MUI.
import type {} from '@mui/x-data-grid/themeAugmentation';
import ErrorBoundary from './components/ErrorBoundary';
import { getToken, logout } from './api/auth';
import type { SessionData } from './api/auth';

const Login = lazy(() => import('./pages/Login'));
const Home = lazy(() => import('./pages/Home'));
const InternetSales = lazy(() => import('./pages/InternetSales'));
const PromoAnalysis = lazy(() => import('./pages/PromoAnalysis'));
const NetworkRegistry = lazy(() => import('./pages/NetworkRegistry'));

const modernTheme = createTheme({
  palette: {
    mode: 'light',
    primary: { 
      main: '#6366f1',
      light: '#818cf8',
      dark: '#4f46e5',
      contrastText: '#ffffff',
    },
    background: { default: '#f8fafc', paper: '#ffffff' },
    text: { primary: '#0f172a', secondary: '#64748b' },
    divider: '#e2e8f0',
  },
  typography: {
    fontFamily: '"Inter", "Helvetica", "Arial", sans-serif',
    fontSize: 14,
    h3: { fontWeight: 700, letterSpacing: '-0.02em', color: '#0f172a' },
    h5: { fontWeight: 600, letterSpacing: '-0.01em', color: '#0f172a' },
    h6: { fontWeight: 600, letterSpacing: '-0.01em', color: '#0f172a' },
    subtitle1: { fontWeight: 600 },
    subtitle2: { fontWeight: 600 },
    button: { textTransform: 'none', fontWeight: 600, letterSpacing: '0em' },
  },
  shape: { borderRadius: 12 },
  components: {
    MuiButton: {
      styleOverrides: {
        root: {
          borderRadius: 10,
          padding: '8px 20px',
          boxShadow: 'none',
          transition: 'all 0.2s ease-in-out',
          '&:hover': { boxShadow: 'none', transform: 'translateY(-1px)' }
        },
        contained: {
          boxShadow: '0 4px 6px -1px rgba(99, 102, 241, 0.2), 0 2px 4px -2px rgba(99, 102, 241, 0.2)',
          '&:hover': { boxShadow: '0 10px 15px -3px rgba(99, 102, 241, 0.3), 0 4px 6px -4px rgba(99, 102, 241, 0.3)' }
        }
      }
    },
    MuiPaper: {
      styleOverrides: {
        root: { backgroundImage: 'none' },
        elevation1: { boxShadow: '0 1px 3px 0 rgba(0, 0, 0, 0.1), 0 1px 2px -1px rgba(0, 0, 0, 0.1)' },
        elevation2: { boxShadow: '0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -2px rgba(0, 0, 0, 0.1)' },
        outlined: { borderColor: '#e2e8f0', borderRadius: 16 },
      }
    },
    MuiInputLabel: {
      styleOverrides: {
        root: {
          color: '#475569',
          '&.Mui-focused': { color: '#6366f1' },
        },
        shrink: { color: '#6366f1' },
      },
    },
    MuiTextField: { defaultProps: { size: 'small' } },
    MuiOutlinedInput: {
      styleOverrides: {
        root: {
          borderRadius: 8,
          backgroundColor: '#ffffff',
          transition: 'all 0.2s',
          '&:hover .MuiOutlinedInput-notchedOutline': { borderColor: '#94a3b8' },
          '&.Mui-focused .MuiOutlinedInput-notchedOutline': { borderWidth: '1px' },
        },
        notchedOutline: { borderColor: '#cbd5e1' },
      }
    },
    MuiDataGrid: {
      styleOverrides: {
        root: {
          border: 'none',
          backgroundColor: '#ffffff',
          boxShadow: '0 4px 6px -1px rgba(0, 0, 0, 0.05), 0 2px 4px -2px rgba(0, 0, 0, 0.05)',
          borderRadius: 16,
          overflow: 'hidden',
          '& .MuiDataGrid-columnHeaders': { backgroundColor: '#f1f5f9', borderBottom: '1px solid #e2e8f0' },
          '& .MuiDataGrid-cell': { borderBottom: '1px solid #f8fafc' },
          '& .MuiDataGrid-row:hover': { backgroundColor: '#f1f5f9' },
          '& .MuiDataGrid-columnSeparator': { display: 'none' },
        },
      },
    },
    MuiDialog: {
      styleOverrides: {
        paper: { borderRadius: 20, boxShadow: '0 25px 50px -12px rgba(0, 0, 0, 0.25)' }
      }
    },
    MuiTabs: {
      styleOverrides: { indicator: { height: 3, borderRadius: '3px 3px 0 0' } }
    },
    MuiTab: {
      styleOverrides: { root: { textTransform: 'none', fontWeight: 600, fontSize: '0.95rem' } }
    }
  },
});

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
