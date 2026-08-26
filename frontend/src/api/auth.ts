export interface SessionData {
  token?: string;
  username?: string;
  role?: string;
  // Как подписывать пользователя: у КАМа — имя из справочника, у остальных
  // логин. Логин собран транслитерацией и читается плохо.
  display_name?: string;
}

export const getToken = (): string | null => localStorage.getItem('token');
export const getUsername = (): string | null => localStorage.getItem('username');
export const getRole = (): string | null => localStorage.getItem('role');
export const getDisplayName = (): string | null =>
  localStorage.getItem('display_name') || localStorage.getItem('username');

export const logout = (): void => {
	void fetch(`${import.meta.env.VITE_API_BASE || 'http://localhost:8080'}/api/auth/logout`, {
		method: 'POST',
		credentials: 'include',
	}).catch(() => {
		// Локальный logout не должен зависеть от доступности backend.
	});
	localStorage.removeItem('token');
  localStorage.removeItem('username');
  localStorage.removeItem('role');
  localStorage.removeItem('display_name');
  window.dispatchEvent(new CustomEvent('auth:logout'));
};

// Сохраняет данные сессии из ответа логина/рефреша
export const saveSession = (data: SessionData): void => {
  if (data.token) localStorage.setItem('token', data.token);
  if (data.username) localStorage.setItem('username', data.username);
  if (data.role) localStorage.setItem('role', data.role);
  if (data.display_name) localStorage.setItem('display_name', data.display_name);
};

// Пытается обновить access token через refresh cookie
// Возвращает true если успешно
export const refreshToken = async (): Promise<boolean> => {
  try {
    const res = await fetch(`${import.meta.env.VITE_API_BASE || 'http://localhost:8080'}/api/auth/refresh`, {
      method: 'POST',
      credentials: 'include', // отправляем httpOnly cookie
    });
    if (!res.ok) return false;
    const data: SessionData = await res.json();
    saveSession(data);
    return true;
  } catch {
    return false;
  }
};
