### Tabel RAM: Payload 100MB (MB)

| Segmen | REST P50 | REST P95 | REST P99 | gRPC P50 | gRPC P95 | gRPC P99 | Rasio P50 | Rasio P95 | Rasio P99 |
|---|---|---|---|---|---|---|---|---|---|
| Client ke Gateway | 758.86 | 997.49 | 1018.70 | 384.02 | 499.23 | 509.47 | 0.51× | 0.50× | 0.50× |
| Gateway ke Worker | 384.00 | 499.20 | 509.44 | 384.00 | 499.20 | 509.44 | 1.00× | 1.00× | 1.00× |
| Worker ke Gateway | — | — | — | — | — | — | — | — | — |
| Gateway ke Client | 738.46 | 995.45 | 1018.29 | 384.00 | 499.20 | 509.44 | 0.52× | 0.50× | 0.50× |
