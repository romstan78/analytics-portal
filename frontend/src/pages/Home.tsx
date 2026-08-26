import type { ReactElement } from 'react';
import { useNavigate } from 'react-router-dom';
import { Box, Typography, Card, CardActionArea, Button, Chip } from '@mui/material';
import { getDisplayName } from '../api/auth';
import {
  BarChart as BarChartIcon,
  ListAlt as ListAltIcon,
  ShoppingCart as CartIcon,
  Refresh as RefreshIcon,
  Campaign as CampaignIcon,
  CompareArrows as CompareIcon,
  ScheduleOutlined as SoonIcon,
} from '@mui/icons-material';

interface HomeBlock {
  title: string;
  path: string;
  icon: ReactElement;
  desc: string;
  color: string;
  ready: boolean;
}

const blocks: HomeBlock[] = [
  {
    title: 'Продажи',
    path: '/internet-sales',
    icon: <CartIcon sx={{ fontSize: 48 }} />,
    desc: 'Детализация онлайн и оффлайн продаж',
    color: '#f59e0b',
    ready: true,
  },
  {
    title: 'Реестр сетей',
    path: '/network-registry',
    icon: <ListAltIcon sx={{ fontSize: 48 }} />,
    desc: 'План-Факт-Прогноз по аптечным сетям',
    color: '#10b981',
    ready: true,
  },
  {
    title: 'Анализ промо',
    path: '/promo-analysis',
    icon: <CampaignIcon sx={{ fontSize: 48 }} />,
    desc: 'Эффективность промо-акций',
    color: '#f43f5e',
    ready: true,
  },
  {
    title: 'Анализ продаж',
    path: '/sales-analysis',
    icon: <BarChartIcon sx={{ fontSize: 36 }} />,
    desc: 'Динамика продаж по периодам',
    color: '#6366f1',
    ready: false,
  },
  {
    title: 'Оборачиваемость',
    path: '/turnover',
    icon: <RefreshIcon sx={{ fontSize: 36 }} />,
    desc: 'Анализ оборачиваемости по АС',
    color: '#8b5cf6',
    ready: false,
  },
  {
    title: 'Продажи Like For Like',
    path: '/like-for-like',
    icon: <CompareIcon sx={{ fontSize: 36 }} />,
    desc: 'Сравнение продаж LFL',
    color: '#0ea5e9',
    ready: false,
  },
];

const readyBlocks = blocks.filter((b) => b.ready);
const soonBlocks = blocks.filter((b) => !b.ready);

interface HomeProps {
  onLogout: () => void;
}

export default function Home({ onLogout }: HomeProps) {
  const navigate = useNavigate();

  return (
    <Box sx={{ p: { xs: 3, md: 6 }, maxWidth: 1400, mx: 'auto', w: '100%' }}>
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', mb: 1 }}>
        <Box>
          <Typography variant="h3" gutterBottom>
            Аналитический портал
          </Typography>
          <Typography variant="subtitle1" color="text.secondary" sx={{ mb: 5 }}>
            Добро пожаловать. Выберите нужный раздел для начала работы.
          </Typography>
        </Box>
        {onLogout && (
          <Button
            variant="outlined"
            onClick={onLogout}
            size="small"
            sx={{ mt: 1 }}
          >
            Выйти ({getDisplayName()})
          </Button>
        )}
      </Box>

      <Typography variant="overline" sx={{ color: 'text.secondary', fontWeight: 700, letterSpacing: '0.08em' }}>
        Разделы
      </Typography>
      <Box sx={{
        display: 'grid',
        gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr', md: '1fr 1fr 1fr' },
        gap: 3,
        mt: 1.5,
        mb: 6,
      }}>
        {readyBlocks.map((block) => (
          <Card
            key={block.path}
            elevation={1}
            sx={{
              borderRadius: 5,
              transition: 'all 0.3s cubic-bezier(0.4, 0, 0.2, 1)',
              border: '1px solid #f1f5f9',
              '&:hover': {
                transform: 'translateY(-6px)',
                boxShadow: '0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 8px 10px -6px rgba(0, 0, 0, 0.1)',
                borderColor: 'transparent'
              }
            }}
          >
            <CardActionArea
              onClick={() => navigate(block.path)}
              sx={{ p: { xs: 3, md: 4.5 }, display: 'flex', flexDirection: 'column', alignItems: 'center', textAlign: 'center' }}
            >
              <Box
                sx={{
                  width: 88,
                  height: 88,
                  borderRadius: '22px',
                  mb: 3,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  backgroundColor: `${block.color}15`,
                  color: block.color,
                }}
              >
                {block.icon}
              </Box>
              <Typography variant="h5" gutterBottom sx={{ fontWeight: 700 }}>
                {block.title}
              </Typography>
              <Typography variant="body1" color="text.secondary" sx={{ lineHeight: 1.6 }}>
                {block.desc}
              </Typography>
            </CardActionArea>
          </Card>
        ))}
      </Box>

      <Typography variant="overline" sx={{ color: 'text.disabled', fontWeight: 700, letterSpacing: '0.08em' }}>
        В разработке
      </Typography>
      <Box sx={{
        display: 'grid',
        gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr', md: '1fr 1fr 1fr' },
        gap: 2,
        mt: 1.5,
      }}>
        {soonBlocks.map((block) => (
          <Card
            key={block.path}
            elevation={0}
            sx={{
              borderRadius: 4,
              border: '1px dashed #e2e8f0',
              backgroundColor: 'transparent',
              transition: 'all 0.2s ease-in-out',
              '&:hover': {
                borderColor: '#cbd5e1',
                backgroundColor: '#f8fafc',
              }
            }}
          >
            <CardActionArea
              onClick={() => navigate(block.path)}
              sx={{ p: 2.5, display: 'flex', alignItems: 'center', gap: 2, textAlign: 'left' }}
            >
              <Box
                sx={{
                  width: 52,
                  height: 52,
                  flexShrink: 0,
                  borderRadius: '14px',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  backgroundColor: '#f1f5f9',
                  color: 'text.disabled',
                }}
              >
                {block.icon}
              </Box>
              <Box sx={{ minWidth: 0, flex: 1 }}>
                <Typography variant="subtitle1" sx={{ fontWeight: 600, color: 'text.secondary' }} noWrap>
                  {block.title}
                </Typography>
                <Typography variant="body2" color="text.disabled" noWrap>
                  {block.desc}
                </Typography>
              </Box>
              <Chip
                icon={<SoonIcon sx={{ fontSize: 16 }} />}
                label="Скоро"
                size="small"
                sx={{ flexShrink: 0, color: 'text.disabled', borderColor: '#e2e8f0' }}
                variant="outlined"
              />
            </CardActionArea>
          </Card>
        ))}
      </Box>
    </Box>
  );
}