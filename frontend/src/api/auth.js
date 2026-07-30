export const getToken = () => localStorage.getItem('token');
export const getUsername = () => localStorage.getItem('username');
export const getRole = () => localStorage.getItem('role');

export const logout = () => {
  localStorage.removeItem('token');
  localStorage.removeItem('username');
  localStorage.removeItem('role');
};