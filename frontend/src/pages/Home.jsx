import { useNavigate } from 'react-router-dom';
import { Box, Typography, Grid, Card, CardActionArea, CardContent, Button } from '@mui/material';
import { 
  BarChart as BarChartIcon, 
  ListAlt as ListAltIcon, 
  ShoppingCart as CartIcon, 
  Refresh as RefreshIcon,
  Campaign as CampaignIcon,
  CompareArrows as CompareIcon,
} from '@mui/icons-material';

const blocks = [
  { 
    title: 'Анализ продаж', 
    path: '/sales-analysis', 
    icon: <BarChartIcon sx={{ fontSize: 36 }} />, 
    desc: 'Динамика продаж по периодам',
    color: '#6366f1',
  },
  { 
    title: 'Реестр сетей', 
    path: '/network-registry', 
    icon: <ListAltIcon sx={{ fontSize: 36 }} />, 
    desc: 'Справочник торговых сетей',
    color: '#10b981',
  },
  { 
    title: 'Интернет-продажи', 
    path: '/internet-sales', 
    icon: <CartIcon sx={{ fontSize: 36 }} />, 
    desc: 'Детализация онлайн-заказов',
    color: '#f59e0b',
  },
  { 
    title: 'Оборачиваемость', 
    path: '/turnover', 
    icon: <RefreshIcon sx={{ fontSize: 36 }} />, 
    desc: 'Анализ оборотов запасов',
    color: '#8b5cf6',
  },
  { 
    title: 'Анализ промо', 
    path: '/promo-analysis', 
    icon: <CampaignIcon sx={{ fontSize: 36 }} />, 
    desc: 'Эффективность промо-акций',
    color: '#f43f5e',
  },
  { 
    title: 'Продажи Like For Like', 
    path: '/like-for-like', 
    icon: <CompareIcon sx={{ fontSize: 36 }} />, 
    desc: 'Сравнение продаж LFL',
    color: '#0ea5e9',
  },
];

export default function Home({ onLogout }) {
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
            Выйти ({localStorage.getItem('username')})
          </Button>
        )}
      </Box>
      
      <Grid container spacing={4}>
        {blocks.map((block) => (
          <Grid item xs={12} sm={6} md={4} key={block.path}>
            <Card 
              elevation={1} 
              sx={{ 
                height: '100%', 
                borderRadius: 4,
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
                sx={{ height: '100%', p: 3, display: 'flex', flexDirection: 'column', alignItems: 'flex-start' }}
              >
                <CardContent sx={{ textAlign: 'left', p: 0, w: '100%' }}>
                  <Box 
                    sx={{ 
                      width: 64, 
                      height: 64, 
                      borderRadius: '16px', 
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
                </CardContent>
              </CardActionArea>
            </Card>
          </Grid>
        ))}
      </Grid>
    </Box>
  );
}