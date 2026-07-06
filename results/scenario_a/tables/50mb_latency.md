### Tabel Latency: Payload 50MB (ms)

| Segmen | REST P50 | REST P95 | REST P99 | gRPC P50 | gRPC P95 | gRPC P99 | Rasio P50 | Rasio P95 | Rasio P99 |
|---|---|---|---|---|---|---|---|---|---|
| Client ke Gateway | 76.85 | 141.25 | 228.25 | 175.00 | 242.50 | 248.50 | 2.28× | 1.72× | 1.09× |
| Gateway ke Worker | 92.65 | 231.88 | 246.38 | 175.00 | 242.50 | 248.50 | 1.89× | 1.05× | 1.01× |
| Worker ke Gateway | 17.77 | 24.76 | 42.75 | 38.94 | 75.83 | 95.17 | 2.19× | 3.06× | 2.23× |
| Gateway ke Client | 8.45 | 24.04 | 2065.00 | 39.50 | 81.87 | 96.38 | 4.67× | 3.41× | 0.05× |
