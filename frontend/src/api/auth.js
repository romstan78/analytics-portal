export const getToken = () => localStorage.getItem('token');
export const getUsername = () => localStorage.getItem('username');
export const getRole = () => localStorage.getItem('role');

export const logout = () => {
  localStorage.removeItem('token');
  localStorage.removeItem('username');
  localStorage.removeItem('role');
};

// Сохраняет данные сессии из ответа логина/рефреша
export const saveSession = (data) => {
  if (data.token) localStorage.setItem('token', data.token);
  if (data.username) localStorage.setItem('username', data.username);
  if (data.role) localStorage.setItem('role', data.role);
};

// Пытается обновить access token через refresh cookie
// Возвращает true если успешно
export const refreshToken = async () => {
  try {
    const res = await fetch('/api/auth/refresh', {
      method: 'POST',
      credentials: 'include', // отправляем httpOnly cookie
    });
    if (!res.ok) return false;
    const data = await res.json();
    saveSession(data);
    return true;
  } catch {
    return false;
  }
};