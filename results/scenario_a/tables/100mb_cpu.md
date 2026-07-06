### Tabel CPU: Payload 100MB (ratio/core)

| Segmen | REST P50 | REST P95 | REST P99 | gRPC P50 | gRPC P95 | gRPC P99 | Rasio P50 | Rasio P95 | Rasio P99 |
|---|---|---|---|---|---|---|---|---|---|
| Client ke Gateway | 1.64 | 1.88 | 1.98 | 2.11 | 2.25 | 2.43 | 1.29× | 1.20× | 1.23× |
| Gateway ke Worker | 1.79 | 1.98 | 2.00 | 2.12 | 2.24 | 2.25 | 1.18× | 1.13× | 1.12× |
| Worker ke Gateway | — | — | — | — | — | — | — | — | — |
| Gateway ke Client | 1.05 | 1.94 | 2.18 | 1.38 | 1.57 | 1.71 | 1.31× | 0.81× | 0.78× |
