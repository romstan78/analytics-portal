import { useState } from 'react';
import { Routes, Route, useNavigate } from 'react-router-dom';
import { createTheme, ThemeProvider, CssBaseline, Box, Typography, Button } from '@mui/material';
import { ArrowBack as ArrowBackIcon } from '@mui/icons-material';
import Login from './pages/Login';
import Home from './pages/Home';
import InternetSales from './pages/InternetSales';
import PromoAnalysis from './pages/PromoAnalysis';
import { getToken, logout } from './api/auth';

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

function PlaceholderPage({ title, description }) {
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

export default function App() {
  const [auth, setAuth] = useState(() => ({
    token: getToken(),
    username: localStorage.getItem('username'),
    role: localStorage.getItem('role'),
  }));

  const handleLogin = (data) => {
    setAuth({ token: data.token, username: data.username, role: data.role });
  };

  const handleLogout = () => {
    logout();
    setAuth({ token: null, username: null, role: null });
  };

  if (!auth.token) {
    return (
      <ThemeProvider theme={modernTheme}>
        <CssBaseline />
        <Login onLogin={handleLogin} />
      </ThemeProvider>
    );
  }

  return (
    <ThemeProvider theme={modernTheme}>
      <CssBaseline />
      <Box sx={{ display: 'flex', flexDirection: 'column', minHeight: '100vh', bgcolor: 'background.default' }}>
        <Routes>
          <Route path="/" element={<Home onLogout={handleLogout} />} />
          <Route path="/internet-sales" element={<InternetSales />} />
          <Route path="/promo-analysis" element={<PromoAnalysis role={auth.role} />} />
          <Route path="/sales-analysis" element={<PlaceholderPage title="Анализ продаж" description="Динамика продаж по периодам" />} />
          <Route path="/network-registry" element={<PlaceholderPage title="Реестр сетей" description="Справочник торговых сетей" />} />
          <Route path="/turnover" element={<PlaceholderPage title="Оборачиваемость" description="Анализ оборотов запасов" />} />
          <Route path="/like-for-like" element={<PlaceholderPage title="Продажи Like For Like" description="Сравнение продаж LFL" />} />
        </Routes>
      </Box>
    </ThemeProvider>
  );
}