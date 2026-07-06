### Tabel CPU: Payload 50MB (ratio/core)

| Segmen | REST P50 | REST P95 | REST P99 | gRPC P50 | gRPC P95 | gRPC P99 | Rasio P50 | Rasio P95 | Rasio P99 |
|---|---|---|---|---|---|---|---|---|---|
| Client ke Gateway | 1.74 | 1.97 | 1.99 | 2.12 | 2.24 | 2.25 | 1.22× | 1.14× | 1.13× |
| Gateway ke Worker | 2.13 | 2.41 | 2.48 | 2.35 | 2.49 | 2.50 | 1.10× | 1.03× | 1.01× |
| Worker ke Gateway | — | — | — | — | — | — | — | — | — |
| Gateway ke Client | 0.00 | 2.64 | 4.43 | 1.57 | 1.94 | 1.99 | — | 0.73× | 0.45× |
