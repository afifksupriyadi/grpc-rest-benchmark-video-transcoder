### Tabel RAM: Payload 50MB (MB)

| Segmen | REST P50 | REST P95 | REST P99 | gRPC P50 | gRPC P95 | gRPC P99 | Rasio P50 | Rasio P95 | Rasio P99 |
|---|---|---|---|---|---|---|---|---|---|
| Client ke Gateway | 384.00 | 499.20 | 509.44 | 97.20 | 127.09 | 225.53 | 0.25× | 0.25× | 0.44× |
| Gateway ke Worker | 192.00 | 249.60 | 254.72 | 192.00 | 249.60 | 254.72 | 1.00× | 1.00× | 1.00× |
| Worker ke Gateway | — | — | — | — | — | — | — | — | — |
| Gateway ke Client | 225.68 | 474.88 | 504.58 | 132.48 | 243.65 | 253.53 | 0.59× | 0.51× | 0.50× |
