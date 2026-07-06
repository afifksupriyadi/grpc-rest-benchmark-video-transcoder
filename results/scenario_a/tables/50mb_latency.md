### Tabel Latency: Payload 50MB (ms)

| Segmen | REST P50 | REST P95 | REST P99 | gRPC P50 | gRPC P95 | gRPC P99 | Rasio P50 | Rasio P95 | Rasio P99 |
|---|---|---|---|---|---|---|---|---|---|
| Client ke Gateway | 79.80 | 127.91 | 177.50 | 172.50 | 271.77 | 287.90 | 2.16× | 2.12× | 1.62× |
| Gateway ke Worker | 102.74 | 148.40 | 186.06 | 168.94 | 263.71 | 286.29 | 1.64× | 1.78× | 1.54× |
| Worker ke Gateway | 15.24 | 24.08 | 25.32 | 38.04 | 55.68 | 57.27 | 2.50× | 2.31× | 2.26× |
| Gateway ke Client | 6.73 | 19.44 | 24.39 | 47.22 | 65.59 | 82.32 | 7.02× | 3.37× | 3.38× |
