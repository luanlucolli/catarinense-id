# Auth Catarinense

## Envs recomendadas para Neon

```env
DATABASE_URL="postgresql://..."
```

O projeto usa defaults fixos otimizados para Neon Free Tier:

- `MaxConns=4`
- `MinConns=0`
- `MaxConnIdleTime=5m`
- `MaxConnLifetime=30m`
- `MaxConnLifetimeJitter=5m`
- `HealthCheckPeriod=1m`
- `ConnectTimeout=10s`
- `DATABASE_STARTUP_TIMEOUT=40s`
- `SESSION_TOUCH_INTERVAL=5m`
