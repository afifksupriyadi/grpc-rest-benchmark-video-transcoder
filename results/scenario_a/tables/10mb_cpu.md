### Tabel Cpu — Payload 10MB (ratio/core)

| Segmen | REST P50 | REST P95 | REST P99 | gRPC P50 | gRPC P95 | gRPC P99 | Rasio P50 | Rasio P95 | Rasio P99 |
|---|---|---|---|---|---|---|---|---|---|
| Client ke Gateway | 1.81 | 2.48 | 2.67 | 2.10 | 2.44 | 2.49 | 1.16× | 0.98× | 0.93× |
| Gateway ke Worker | 1.89 | 2.21 | 2.24 | 2.35 | 2.81 | 2.96 | 1.24× | 1.27× | 1.32× |
| Worker ke Gateway | — | — | — | — | — | — | — | — | — |
| Gateway ke Client | 0.00 | 7.61 | 7.75 | 1.60 | 2.45 | 2.92 | — | 0.32× | 0.38× |
