import { useState } from 'react';
import { Box, Card, TextField, Button, Typography, Alert } from '@mui/material';
import { Lock as LockIcon } from '@mui/icons-material';

export default function Login({ onLogin }) {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!username || !password) {
      setError('Заполните все поля');
      return;
    }
    setLoading(true); setError('');
    try {
      const response = await fetch('http://localhost:8080/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password }),
      });
      const data = await response.json();
      if (!response.ok) {
        setError(data.error || 'Ошибка входа');
        return;
      }
      localStorage.setItem('token', data.token);
      localStorage.setItem('username', data.username);
      localStorage.setItem('role', data.role);
      onLogin(data);
    } catch (err) {
      setError('Сервер недоступен');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Box sx={{ 
      minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center',
      bgcolor: '#f1f5f9'
    }}>
      <Card elevation={3} sx={{ p: 5, width: 400, borderRadius: 4 }}>
        <Box sx={{ textAlign: 'center', mb: 3 }}>
          <Box sx={{ 
            width: 56, height: 56, borderRadius: '16px', bgcolor: '#6366f115',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            mx: 'auto', mb: 2, color: '#6366f1'
          }}>
            <LockIcon sx={{ fontSize: 28 }} />
          </Box>
          <Typography variant="h5" sx={{ fontWeight: 700 }}>Вход в систему</Typography>
          <Typography variant="body2" color="text.secondary">Аналитический портал</Typography>
        </Box>

        {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}

        <form onSubmit={handleSubmit}>
          <TextField label="Логин" fullWidth size="small" value={username}
            onChange={(e) => setUsername(e.target.value)} sx={{ mb: 2 }} />
          <TextField label="Пароль" type="password" fullWidth size="small" value={password}
            onChange={(e) => setPassword(e.target.value)} sx={{ mb: 3 }} />
          <Button variant="contained" fullWidth type="submit" disabled={loading}
            sx={{ py: 1.2, fontWeight: 600 }}>
            {loading ? 'Вход...' : 'Войти'}
          </Button>
        </form>
      </Card>
    </Box>
  );
}