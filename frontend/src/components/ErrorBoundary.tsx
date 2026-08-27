// Граница ошибок отрисовки.
//
// Без неё исключение в любом блоке гасит всё дерево React, и пользователь
// видит белый экран без единой подсказки, что произошло и что делать. Так уже
// падал разбор сети: одно поле пришло как null вместо списка.
//
// Класс, а не функция: перехват ошибок отрисовки в React есть только у
// классовых компонентов — хуковой замены getDerivedStateFromError нет.

import { Component } from 'react';
import type { ErrorInfo, ReactNode } from 'react';
import { Box, Button, Paper, Stack, Typography } from '@mui/material';
import RefreshIcon from '@mui/icons-material/Refresh';
import HomeIcon from '@mui/icons-material/Home';

interface ErrorBoundaryProps {
  children: ReactNode;
  // Смена ключа сбрасывает границу. Без этого уход с упавшей страницы
  // оставлял бы сообщение о ней висеть поверх исправной.
  resetKey?: string;
}

interface ErrorBoundaryState {
  error: Error | null;
}

export default class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = { error: null };

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    // В консоль, а не на сервер: приёмки ошибок у портала нет, а проглотить
    // исключение молча — остаться вообще без следа. Стек компонентов важнее
    // самого сообщения: он показывает, какой блок упал.
    console.error('Ошибка интерфейса:', error, info.componentStack);
  }

  componentDidUpdate(prevProps: ErrorBoundaryProps) {
    if (this.state.error && prevProps.resetKey !== this.props.resetKey) {
      this.setState({ error: null });
    }
  }

  render() {
    const { error } = this.state;
    if (!error) return this.props.children;

    return (
      <Box sx={{ display: 'grid', placeItems: 'center', minHeight: '70vh', p: 3 }}>
        <Paper variant="outlined" sx={{ p: 4, maxWidth: 560, borderRadius: 3 }}>
          <Typography variant="h6" sx={{ fontWeight: 750, mb: 1 }}>
            Этот экран не отрисовался
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            Ошибка в интерфейсе, а не в ваших данных: ничего не сохранялось и не
            менялось. Попробуйте открыть экран заново — если повторится, покажите
            разработчику текст ниже.
          </Typography>
          <Box
            component="pre"
            sx={{
              p: 1.5, mb: 2.5, borderRadius: 2, bgcolor: 'grey.100',
              fontSize: 12, whiteSpace: 'pre-wrap', wordBreak: 'break-word',
              maxHeight: 180, overflow: 'auto', m: 0,
            }}
          >
            {error.message || String(error)}
          </Box>
          <Stack direction="row" spacing={1}>
            <Button
              variant="contained"
              startIcon={<RefreshIcon />}
              onClick={() => this.setState({ error: null })}
            >
              Попробовать снова
            </Button>
            {/* Полная перезагрузка, а не переход роутером: если сломался сам
                роутер, мягкий переход упадёт ровно так же. */}
            <Button
              startIcon={<HomeIcon />}
              onClick={() => window.location.assign('/')}
            >
              На главную
            </Button>
          </Stack>
        </Paper>
      </Box>
    );
  }
}
