# Dokumen Keputusan Arsitektur dan Desain Sistem
## Analisis Komparatif Kinerja REST dan gRPC pada Komunikasi Microservices Berbasis Streaming: Studi Kasus Sistem Pemrosesan Video Terdistribusi

**NIM:** 1301223161  
**Nama:** Afif Kurniawan Supriyadi  
**Dokumen ini bersifat internal** — digunakan sebagai acuan teknis selama proses implementasi dan penulisan laporan akhir.

---

## 1. Tujuan Penelitian

Penelitian ini tidak bertujuan sekadar menyimpulkan protokol mana yang lebih cepat. Tujuan yang lebih bermakna adalah **menghasilkan panduan pemilihan protokol berbasis data** — menentukan pada kondisi operasional seperti apa REST lebih sesuai, dan pada kondisi seperti apa gRPC memberikan keunggulan yang signifikan. Hasil akhir penelitian diharapkan dapat menjadi referensi bagi engineer dalam mengambil keputusan arsitektur pada sistem microservices yang menangani transfer data berukuran besar.

---

## 2. Arsitektur Sistem

### 2.1 Alur Komunikasi

```
Client (CLI, luar Docker)
    │
    │ [1] Upload video (client-side streaming)
    ▼
Gateway Service (Docker)
    │
    │ [2] Teruskan video ke Worker (bidirectional streaming)
    ▼
Transcoding Worker (Docker)
    │
    │ [3] Proses FFmpeg (tidak diukur)
    │
    │ [4] Kirim hasil ke Gateway (bidirectional streaming)
    ▼
Gateway Service (Docker)
    │
    │ [5] Teruskan hasil ke Client (server-side streaming)
    ▼
Client (CLI, luar Docker)
    └── Simpan file hasil ke disk lokal
```

### 2.2 Prinsip Desain yang Disepakati

**Tidak ada shared volume antar-service.** Keputusan ini disengaja agar data video benar-benar mengalir melalui jaringan antar-service, sehingga pengukuran REST vs gRPC pada transfer data besar menjadi bermakna secara metodologis. Jika shared volume digunakan, service-service dapat membaca file langsung dari disk tanpa melalui jaringan, sehingga REST dan gRPC hanya membawa pesan kecil berisi nama file — tidak ada payload besar yang terukur.

**Pendekatan buffer, bukan stream piping.** Setiap service menerima seluruh data terlebih dahulu sebelum meneruskan ke service berikutnya. Keputusan ini memastikan setiap segmen komunikasi dapat diukur secara terpisah dan bersih, sehingga isolasi metrik per segmen dapat dilakukan dengan tepat.

**Hanya Gateway yang menyimpan data.** Client menyimpan hasil akhir di disk lokalnya sendiri. Tidak diperlukan object storage eksternal seperti S3 atau OSS karena hal tersebut menambahkan variabel latensi jaringan ke cloud yang tidak dapat dikontrol dan akan mencemari data pengukuran.

### 2.3 Jenis Streaming per Interaksi

| Interaksi | gRPC | REST |
|---|---|---|
| Client → Gateway (upload video) | Client-side streaming | HTTP POST dengan chunked transfer encoding |
| Gateway ↔ Worker (kirim video + terima hasil) | Bidirectional streaming | HTTP POST (Gateway sebagai HTTP client ke Worker) |
| Gateway → Client (download hasil) | Server-side streaming | HTTP response body dengan chunked transfer encoding |

**Catatan untuk gRPC bidirectional streaming (Gateway ↔ Worker):** Satu RPC call yang sama digunakan untuk dua fase secara berurutan dalam satu koneksi — Gateway mengirim video dulu sampai selesai, kemudian Worker mengirim hasil balik melalui koneksi yang sama. Worker tidak pernah membuka koneksi baru ke Gateway.

---

## 3. Komponen Sistem

### 3.1 Komponen yang Digunakan

| Komponen | Peran | Keterangan |
|---|---|---|
| Client CLI | Antarmuka pengguna | Berjalan di luar Docker, dibangun dengan Cobra |
| Gateway Service | Pintu masuk sistem, koordinasi alur | Container Docker, menggunakan Gin (REST) dan gRPC library |
| Transcoding Worker | Pemrosesan video dengan FFmpeg | 2 instance container Docker |
| Prometheus | Penyimpanan semua metrik | Container Docker |
| Grafana | Visualisasi dan analisis data | Container Docker |

### 3.2 Komponen yang Dibuang

Komponen-komponen berikut diputuskan untuk tidak digunakan beserta alasannya:

- **Coordinator Service** — dihapus karena Gateway dapat menangani distribusi tugas secara langsung dengan goroutine
- **Redis** — dihapus karena tidak ada kebutuhan async job queue; koneksi streaming itu sendiri menjadi mekanisme koordinasi
- **asynq** — dihapus karena memerlukan Redis dan memperkenalkan mekanisme distribusi tugas yang tidak relevan dengan variabel penelitian
- **Jaeger dan OpenTelemetry** — dihapus karena kebutuhan pengukuran fase dapat dipenuhi dengan timestamp `time.Now()` di kode Go tanpa distributed tracing system
- **cAdvisor** — dihapus; digantikan oleh `prometheus.NewProcessCollector()` dan `prometheus.NewGoCollector()` yang dapat mengukur CPU dan memori langsung dari dalam kode Go tanpa akses Docker

### 3.3 Stack Teknologi

| Kebutuhan | Teknologi |
|---|---|
| Bahasa pemrograman | Go (seluruh service) |
| REST server | Gin |
| gRPC | Go gRPC library + Protocol Buffers (protoc) |
| CLI Client | Cobra |
| FFmpeg wrapper | ffmpeg-go |
| Observability | Prometheus + Grafana |
| Containerisasi | Docker + Docker Compose |
| Struktur proyek | Monorepo |

---

## 4. Infrastruktur dan Distribusi Beban

### 4.1 Jumlah Worker Instance

Sistem menggunakan **2 instance Transcoding Worker** yang berjalan sebagai container Docker terpisah. Gateway mendistribusikan request ke kedua Worker menggunakan mekanisme round-robin sederhana.

### 4.2 Penanganan Konkurensi di Gateway

Konkurensi di Gateway ditangani secara otomatis oleh Go melalui goroutine — setiap request yang masuk dijalankan di goroutine terpisah sehingga banyak request dapat diproses secara bersamaan tanpa saling memblokir.

### 4.3 Simulasi Concurrent Users

Untuk skenario pengujian dengan banyak pengguna bersamaan, beberapa instance CLI dijalankan secara paralel melalui bash script:

```bash
for i in $(seq 1 10); do
    client --protocol grpc --input file.mp4 --output hasil_$i.mp4 &
done
wait
```

---

## 5. Pemrosesan Video (FFmpeg)

### 5.1 Perilaku Output yang Dinamis

FFmpeg dikonfigurasi untuk menghasilkan semua resolusi di bawah resolusi input secara dinamis:

| Input | Output yang Dihasilkan |
|---|---|
| 1080p | 720p, 480p, 360p |
| 720p | 480p, 360p |
| 480p | 360p |

Input 4K dihindari dalam pengujian karena ukuran filenya yang sangat besar akan membuat durasi setiap run terlalu panjang dan tidak efisien untuk 30 run per skenario.

### 5.2 Paralelisasi FFmpeg

Seluruh resolusi output diproses secara paralel menggunakan goroutine. Timestamp t5 (Worker mulai kirim hasil) baru dicatat setelah **semua** resolusi selesai diproses.

### 5.3 Pengiriman Hasil Multi-Resolusi ke Client

Hasil dari beberapa file resolusi dikirim dalam **satu stream dengan label resolusi per chunk**. Client membuka beberapa file output sekaligus dan mengarahkan setiap chunk ke file yang sesuai berdasarkan labelnya.

```protobuf
message VideoChunk {
    string resolusi = 1;  // "720p", "480p", "360p"
    bytes data = 2;
    bool selesai = 3;
}
```

---

## 6. Skema Pengukuran Metrik

### 6.1 Skema Timestamp (8 Titik)

Delapan timestamp dicatat di kode Go dan diekspor ke Prometheus sebagai penanda fase:

```
t1 = Client mulai kirim video ke Gateway
t2 = Gateway selesai terima video dari Client
t3 = Gateway mulai kirim video ke Worker
t4 = Worker selesai terima video dari Gateway

      ╔══════════════════════════════╗
      ║   FFmpeg berjalan            ║  ← TIDAK DIUKUR
      ╚══════════════════════════════╝

t5 = Worker mulai kirim hasil ke Gateway
t6 = Gateway selesai terima hasil dari Worker
t7 = Gateway mulai kirim hasil ke Client
t8 = Client selesai terima hasil dari Gateway
```

### 6.2 Perhitungan Latency per Segmen

| Segmen | Rumus | Keterangan |
|---|---|---|
| Client → Gateway | t2 - t1 | Latency upload |
| Gateway → Worker | t4 - t3 | Latency transfer ke Worker (inti penelitian) |
| Worker → Gateway | t6 - t5 | Latency transfer dari Worker (inti penelitian) |
| Gateway → Client | t8 - t7 | Latency download |
| **Total latency protokol** | **(t2-t1) + (t4-t3) + (t6-t5) + (t8-t7)** | **FFmpeg tidak termasuk** |

### 6.3 Perhitungan Throughput per Segmen

| Segmen | Rumus |
|---|---|
| Client → Gateway | ukuran_file_input / (t2 - t1) |
| Gateway → Worker | ukuran_file_input / (t4 - t3) |
| Worker → Gateway | ukuran_file_output / (t6 - t5) |
| Gateway → Client | ukuran_file_output / (t8 - t7) |

### 6.4 Pengukuran CPU dan Memori

CPU dan memori diukur dari **dalam kode Go** menggunakan dua komponen Prometheus library:

```go
registry.MustRegister(prometheus.NewProcessCollector(
    prometheus.ProcessCollectorOpts{},
))
registry.MustRegister(prometheus.NewGoCollector())
```

`ProcessCollector` membaca data dari `/proc/self/stat` di sistem operasi Linux — tidak memerlukan akses Docker. Data CPU dan memori tersedia di endpoint `/metrics` masing-masing service.

**Isolasi fase FFmpeg untuk CPU dan memori:** Prometheus mencatat CPU dan memori setiap 1 detik. Setelah eksperimen selesai, data difilter di Grafana berdasarkan rentang waktu:

- **Dimasukkan ke perhitungan:** data pada rentang t1→t4 dan t5→t8
- **Diabaikan:** data pada rentang t4→t5 (FFmpeg berjalan)

### 6.5 Metrik Persentil (P50, P95, P99)

Selain rata-rata, persentil latency dikumpulkan menggunakan Prometheus Histogram:

```go
latencyHistogram := prometheus.NewHistogram(prometheus.HistogramOpts{
    Name:    "segment_latency_seconds",
    Buckets: prometheus.DefBuckets,
})
latencyHistogram.Observe(latency.Seconds())
```

| Persentil | Makna |
|---|---|
| P50 | 50% request selesai di bawah nilai ini — gambaran performa tipikal |
| P95 | 95% request selesai di bawah nilai ini — gambaran performa di bawah beban |
| P99 | 99% request selesai di bawah nilai ini — gambaran kasus terburuk yang masih sering terjadi |

Persentil digunakan untuk mengukur **konsistensi dan stabilitas** protokol, bukan hanya kecepatan rata-rata.

### 6.6 Metrik Efisiensi Resource

```
CPU cost per MB = rata-rata CPU selama transfer / ukuran file (MB)
```

Metrik ini memungkinkan perbandingan efisiensi penggunaan resource secara proporsional terhadap volume data yang ditransfer.

---

## 7. Skenario Pengujian

Setiap skenario dijalankan **30 kali per protokol** untuk menghasilkan data yang cukup representatif secara statistik.

### Skenario 1 — File Tunggal Berukuran Besar

- **Input:** video 1080p
- **Output yang dihasilkan:** 720p, 480p, 360p
- **Jumlah request:** 1 request per run
- **Tujuan analitis:** Mengukur efisiensi protokol pada kondisi sustained transfer — satu koneksi berlangsung lama dengan volume data besar, tanpa tekanan dari banyak koneksi bersamaan

### Skenario 2 — Banyak Request Bersamaan

- **Input:** video 720p (lebih kecil agar waktu proses lebih terkendali)
- **Output yang dihasilkan:** 480p, 360p
- **Jumlah request:** beberapa request bersamaan (simulasi concurrent users)
- **Tujuan analitis:** Mengukur kemampuan protokol menangani konkurensi — bagaimana latency dan CPU berubah seiring bertambahnya jumlah request bersamaan

### Skenario 3 — Variasi Ukuran Payload

- **Input:** file video dengan ukuran bervariasi secara sistematis
- **Ukuran yang diuji:** 10MB, 50MB, 100MB, 250MB, 500MB
- **Jumlah request:** 1 request per ukuran per run
- **Tujuan analitis:** Menemukan titik perpotongan performa REST dan gRPC — pada ukuran payload berapa REST masih kompetitif, dan di atas ukuran berapa gRPC mulai menunjukkan keunggulan yang signifikan

---

## 8. Desain Docker

**Status: Diimplementasikan 2026-07-06 (Fase 11), dengan 1 penyesuaian scope — lihat catatan di bawah.**

Desain akhir (target penuh, sesuai Section 4.1) tetap 2 instance Worker dengan round-robin di Gateway. Namun implementasi Fase 11 sengaja dipecah jadi dua tahap terpisah, atas kesepakatan eksplisit saat sesi Dockerisasi:

1. **Tahap 1 (selesai):** Dockerisasi Gateway + 1 Worker + Prometheus + Grafana dalam satu `docker-compose.yml`, tanpa round-robin. Tujuannya memverifikasi dulu bahwa jalur jaringan Docker bridge (menggantikan loopback `localhost` yang dipakai saat testing manual di WSL) benar-benar bekerja end-to-end, sebelum menambah kompleksitas 2 Worker sekaligus.
2. **Tahap 2 (belum dikerjakan):** Menambah service `worker-2` ke compose dan mengimplementasikan logic round-robin di `gateway/cmd/server/main.go` (temuan F1 dari review kode) — dikerjakan di sesi terpisah setelah Tahap 1 stabil dipakai untuk testing resmi.

Isi `docker-compose.yml` yang sudah berjalan (Tahap 1):

```yaml
services:
  worker:
    build:
      context: .
      dockerfile: worker/Dockerfile
    environment:
      ENV: docker
      REST_PORT: 8091
      GRPC_PORT: 50052
      METRICS_PORT: 2113
    networks:
      - benchmark-net

  gateway:
    build:
      context: .
      dockerfile: gateway/Dockerfile
    environment:
      ENV: docker
      REST_PORT: 8080
      GRPC_PORT: 50051
      METRICS_PORT: 2112
      WORKER_1_REST_ADDR: worker:8091
      WORKER_1_GRPC_ADDR: worker:50052
      HTTP_CLIENT_TIMEOUT: 60s
      MAX_FILE_SIZE_BYTES: 524288000
    ports:
      - "8080:8080"
      - "50051:50051"
      - "2112:2112"
    depends_on:
      - worker
    networks:
      - benchmark-net

  prometheus:
    image: prom/prometheus:v2.53.0
    volumes:
      - ./prometheus.docker.yml:/etc/prometheus/prometheus.yml:ro
      - prometheus-data:/prometheus
    ports:
      - "9090:9090"
    depends_on:
      - gateway
      - worker
    networks:
      - benchmark-net

  grafana:
    image: grafana/grafana:11.6.0
    ports:
      - "3000:3000"
    volumes:
      - grafana-data:/var/lib/grafana
      - ./grafana/provisioning:/etc/grafana/provisioning:ro
    depends_on:
      - prometheus
    networks:
      - benchmark-net

networks:
  benchmark-net:
    driver: bridge

volumes:
  prometheus-data:
  grafana-data:
```

Catatan implementasi penting:

- **Build context = root repo, bukan subdirektori service.** `gateway/Dockerfile` dan `worker/Dockerfile` sama-sama pakai `COPY . .` dari root, karena struktur Go Workspace (`go.work`) mereferensikan modul `gen/` dan `shared/` juga — kalau context dibatasi ke satu subdirektori saja, resolusi `go.work` akan gagal karena direktori yang di-`use` tidak ditemukan.
- **Image Worker pakai `alpine:3.20` + `apk add ffmpeg`** — paket ini otomatis menyertakan `ffprobe` juga (dikonfirmasi ada di `/usr/bin/ffmpeg` dan `/usr/bin/ffprobe` di image final), karena Worker memanggil keduanya lewat `exec.Command`.
- **Config Gateway/Worker di-set lewat `environment:` di compose, bukan file `.env`.** Loader `cleanenv.ReadConfig(".env", conf)` di ketiga service memang sudah otomatis fallback ke `cleanenv.ReadEnv(conf)` kalau file `.env` tidak ditemukan — jadi container bisa langsung pakai environment variable proses tanpa perlu ubah kode maupun bikin `.env` khusus Docker.
- **Client CLI tetap di luar Docker**, sesuai Section 2.1 — jalan `go run` di host seperti biasa, terhubung ke Gateway lewat port yang dipublikasikan (`8080`, `50051`). Port Worker sengaja TIDAK dipublikasikan ke host karena hanya diakses Gateway dan Prometheus lewat network internal `benchmark-net`.
- **`prometheus.docker.yml`** (file baru, terpisah dari `prometheus.yml` lama) memakai target scrape berbasis nama service Docker (`gateway:2112`, `worker:2113`) alih-alih `localhost`. `prometheus.yml` lama dibiarkan tidak berubah untuk alur `go run` manual di host kalau sewaktu-waktu masih dipakai untuk dev cepat.
- **`grafana/provisioning/datasources/prometheus.yml`** (file baru) membuat datasource Prometheus (`http://prometheus:9090`) otomatis ter-set saat container Grafana start, tidak perlu setup ulang manual lewat UI tiap kali container dibuat ulang.
- **Diverifikasi end-to-end:** `docker compose build` sukses, keempat service `running`, target Prometheus UP, transcode test dari Client (host) ke Gateway→Worker (container) sukses dengan log menunjukkan traffic dari IP bridge Docker (`172.24.0.x`) — bukti jalur jaringan riil sudah dipakai, bukan lagi loopback.

Lihat Section 16.5 untuk prosedur lifecycle (kapan restart, kapan biarkan jalan terus) yang disusun khusus untuk stack Docker ini.

---

## 9. Ringkasan Keputusan Kritis

| Keputusan | Pilihan | Alasan |
|---|---|---|
| Shared volume antar-service | Tidak digunakan | Agar data video mengalir lewat jaringan sehingga REST vs gRPC dapat diukur secara bermakna |
| Stream piping vs buffer | Buffer | Memungkinkan isolasi pengukuran per segmen komunikasi |
| Redis / asynq | Tidak digunakan | Menambah variabel yang tidak relevan dengan penelitian |
| cAdvisor | Tidak digunakan | Digantikan ProcessCollector yang lebih presisi dan tidak memerlukan akses Docker |
| Jaeger / OpenTelemetry | Tidak digunakan | timestamp time.Now() sudah cukup untuk kebutuhan isolasi fase |
| Output FFmpeg | Dinamis (multi-resolusi) | Lebih realistis; FFmpeg dikecualikan dari semua perhitungan |
| Jumlah Worker | 2 instance | Mendukung skenario concurrent users |
| Persentil latency | P50, P95, P99 | Mengukur konsistensi, bukan hanya kecepatan rata-rata |
| Storage hasil video | Tidak ada di server | Hasil di-stream langsung ke Client dan disimpan di disk lokal Client |

---

## 10. Struktur Project (Monorepo)

### 10.1 Keputusan Fundamental Struktur

| Keputusan | Pilihan | Alasan |
|---|---|---|
| Monorepo vs poly-repo | Monorepo | Satu repo untuk semua service memastikan validitas perbandingan — perbedaan performa benar-benar berasal dari protokol, bukan perbedaan struktur kode |
| go.mod per service vs satu go.mod | Per service (multi-module) | Isolasi dependency per service; lebih realistis secara microservices |
| Manajemen cross-module | Go Workspace (`go.work`) | Fitur resmi Go sejak 1.18 untuk monorepo multi-module; menggantikan `replace` directive |
| REST dan gRPC | Satu sistem, dual interface | Gateway dan Worker menjalankan REST server dan gRPC server secara bersamaan; Client memilih via flag `--protocol` |
| Terminologi layer | `handler/`, `service/`, `model/` | Konsisten dengan digidoc dan konvensi umum komunitas Go |

**Satu sistem, dual interface** — artinya bukan "sistem REST" dan "sistem gRPC" yang terpisah, melainkan satu sistem yang punya dua pintu masuk. Handler REST dan handler gRPC keduanya memanggil service layer yang sama. Logika bisnis identik untuk kedua protokol.

### 10.2 Struktur Project Lengkap (v2 Final)

```
ta-research/
├── go.work                                  ← mendaftarkan semua module
├── go.work.sum
├── Makefile                                 ← perintah: proto gen, build, docker
├── docker-compose.yml
├── prometheus.yml
│
├── proto/                                   ← source .proto, BUKAN Go module
│   └── video.proto
│
├── gen/                                     ← generated code, module tersendiri
│   ├── go.mod
│   └── video/
│       ├── video.pb.go                      ← generated by protoc, jangan diedit manual
│       └── video_grpc.pb.go
│
├── shared/                                  ← shared utilities, module tersendiri
│   ├── go.mod
│   └── response/
│       ├── code.go                          ← ErrorCode, SuccessCode constants
│       ├── dict.go                          ← map code → {message, httpStatus}
│       ├── model.go                         ← Response, Body, AppError, ParsedError
│       ├── error.go                         ← WrapAppError, ParseErrorWithHTTP, ParseErrorWithGRPC
│       └── builder.go                       ← BuildSuccess, BuildError
│
├── gateway/
│   ├── go.mod
│   ├── Dockerfile
│   ├── cmd/
│   │   └── server/
│   │       └── main.go                      ← entry point: load config, DI, start REST + gRPC server
│   ├── config/
│   │   └── config.go
│   ├── lib/
│   │   └── metrics/
│   │       ├── metrics.go                   ← interface MetricsRecorder
│   │       ├── prometheus.go                ← implementasi Prometheus
│   │       └── nil_metrics.go              ← no-op untuk testing
│   └── internal/
│       ├── constant/
│       │   ├── resolution.go               ← "720p", "480p", "360p"
│       │   ├── segment.go                  ← nama segmen transfer
│       │   └── protocol.go                 ← "rest", "grpc"
│       ├── handler/
│       │   ├── rest/
│       │   │   ├── video_handler.go        ← Gin handler
│       │   │   └── routes.go
│       │   └── grpc/
│       │       └── video_handler.go        ← implements VideoServiceServer dari gen/
│       ├── service/
│       │   ├── interface.go                ← interface VideoService; interface WorkerClient
│       │   └── video_service.go
│       ├── model/
│       │   ├── video.go                    ← domain structs (tidak punya JSON tag)
│       │   └── video_dto.go               ← REST DTOs (punya JSON tag)
│       ├── worker/                         ← client code Gateway untuk memanggil Worker
│       │   ├── rest/
│       │   │   └── client.go
│       │   └── grpc/
│       │       └── client.go
│       └── util/
│           └── contextutil/
│               └── contextutil.go          ← context keys: t1, t2, t3, t4, protocol
│
├── worker/
│   ├── go.mod
│   ├── Dockerfile
│   ├── cmd/
│   │   └── server/
│   │       └── main.go
│   ├── config/
│   │   └── config.go
│   ├── lib/
│   │   └── metrics/
│   │       ├── metrics.go
│   │       ├── prometheus.go
│   │       └── nil_metrics.go
│   └── internal/
│       ├── constant/
│       │   ├── resolution.go
│       │   ├── ffmpeg.go                   ← codec, preset, format values
│       │   └── segment.go
│       ├── handler/
│       │   ├── rest/
│       │   │   ├── video_handler.go
│       │   │   └── routes.go
│       │   └── grpc/
│       │       └── video_handler.go
│       ├── service/
│       │   ├── interface.go                ← interface VideoService; interface Processor
│       │   └── video_service.go
│       ├── model/
│       │   ├── video.go
│       │   └── video_dto.go
│       ├── processor/
│       │   └── ffmpeg.go                   ← wrapper ffmpeg-go
│       └── util/
│           └── contextutil/
│               └── contextutil.go          ← context keys: t4, t5
│
└── client/
    ├── go.mod
    ├── main.go                              ← cmd.Execute()
    ├── cmd/
    │   ├── root.go                          ← Cobra root: flag --protocol, --input, --output, --addr
    │   └── transcode.go                     ← subcommand: catat t1, panggil transport, catat t8
    ├── config/
    │   └── config.go
    └── internal/
        ├── constant/
        │   ├── protocol.go                  ← "rest", "grpc"
        │   └── default.go                   ← default timeout, default address, default output dir
        ├── transport/
        │   ├── interface.go                 ← interface GatewayClient
        │   ├── rest/
        │   │   └── client.go
        │   └── grpc/
        │       └── client.go
        └── model/
            └── video.go
```

**`go.work`:**
```
go 1.23

use (
    ./gen
    ./shared
    ./gateway
    ./worker
    ./client
)
```

### 10.3 Konvensi dan Aturan per Layer

**Aturan yang berlaku untuk seluruh codebase, mengikuti pola digidoc:**

`handler/` — hanya berisi: validasi input via `SanitizeAndValidate`, pemanggilan service, konversi error ke response. Tidak ada business logic.

`service/` — hanya berisi business logic. Tidak ada akses langsung ke transport layer (tidak tahu apakah dipanggil dari REST atau gRPC). Selalu menerima dan mengembalikan domain struct dari `model/video.go`, bukan proto struct atau JSON struct.

`model/video.go` — domain structs tanpa JSON tag, bukan milik protokol manapun. Ini yang beredar di service layer.

`model/video_dto.go` — REST DTOs dengan JSON tag. Dipakai oleh REST handler untuk binding request dan menulis response. gRPC tidak butuh DTO terpisah karena proto-generated struct di `gen/` sudah menjalankan peran itu.

`constant/` — semua nilai string/angka yang penting disimpan di sini. Tidak ada string hardcoded di handler, service, atau model. Tidak membuat custom type untuk constant — cukup `const (...) string = "..."`.

`lib/metrics/` — mengikuti pola Interface + NilImplementation + ConcreteImplementation dari digidoc. Interface `MetricsRecorder` memungkinkan testing tanpa Prometheus yang berjalan.

`internal/worker/` (di gateway) — kode yang digunakan Gateway untuk memanggil Worker. Setara dengan `internal/adapter/` di digidoc. Dipisahkan menjadi `rest/` dan `grpc/` karena cara memanggil Worker berbeda untuk setiap protokol.

`internal/transport/` (di client) — kode yang digunakan Client untuk memanggil Gateway. Interface `GatewayClient` memungkinkan `cmd/transcode.go` mengganti implementasi secara runtime berdasarkan flag `--protocol` tanpa perlu tahu detail protokolnya.

`util/contextutil/` — per-service, bukan shared. Context tidak melintas antar service. Gateway punya keys t1–t4, Worker punya keys t4–t5.

### 10.4 Error Handling

Mengikuti pola dua lapisan dari digidoc:

**Lapisan 1 — Application Error** (service layer ke bawah):
```go
return nil, response.WrapAppError(ctx, err, response.ErrVideoTooLarge, "video melebihi batas ukuran")
```
Menghasilkan `*AppError` yang membawa error code, original error, lokasi pemanggil, dan optional payload. Auto-log saat dibuat.

**Lapisan 2a — REST Response Error** (REST handler):
```go
return response.BuildError(ctx, err), nil
```
`BuildError` → `ParseErrorWithHTTP` → lookup dict → HTTP status code + response body.

**Lapisan 2b — gRPC Status Error** (gRPC handler):
```go
return nil, response.ParseErrorWithGRPC(err)
```
`ParseErrorWithGRPC` → lookup dict → mapping HTTP status ke gRPC code → `status.Error(grpcCode, message)`.

Mapping HTTP ke gRPC code:
- HTTP 400 → `codes.InvalidArgument`
- HTTP 404 → `codes.NotFound`
- HTTP 500 → `codes.Internal`
- HTTP 503 → `codes.Unavailable`

Package `shared/response/` adalah module tersendiri yang di-import oleh gateway dan worker via Go Workspace.

### 10.5 Perbedaan gRPC Handler vs REST Handler

Perbedaan teknis yang perlu dipahami meski terminologi foldernya sama:

**REST handler (`handler/rest/`):**
```go
// fungsi biasa yang dipasang ke Gin router
func (h *VideoHandler) Upload(c *gin.Context) {
    // bind request → panggil service → tulis response
}
```

**gRPC handler (`handler/grpc/`):**
```go
// struct yang mengimplementasikan interface generated oleh protoc
type VideoServer struct {
    pb.UnimplementedVideoServiceServer
    svc service.VideoService
}

func (s *VideoServer) UploadVideo(stream pb.VideoService_UploadVideoServer) error {
    // terima stream → konversi ke domain struct → panggil service → kirim response stream
}
```

gRPC tidak punya router — routing ditangani otomatis oleh generated code. Yang perlu dilakukan hanya register server ke `grpc.Server`.

### 10.6 Referensi yang Digunakan

Struktur project diputuskan berdasarkan kombinasi beberapa referensi:

- **digidoc** (project magang) — referensi utama untuk pola handler/service/model, error handling, lib pattern, dependency injection, dan constant convention
- **Nathan Smith — Structuring Go gRPC microservices (Medium)** — prinsip gRPC sebagai transport layer only; handler gRPC hanya mapping, bukan business logic
- **steven-vox/distributed-microservice-with-golang (GitHub)** — monorepo Go dengan proto/ di root dan services/ terpisah
- **golang-standards/project-layout (GitHub)** — konvensi penamaan folder umum komunitas Go
- **gRPC Microservices in Go — Hüseyin Babal (Manning)** — referensi buku yang sedang dibaca; hexagonal architecture untuk gRPC service

---

## 11. Pola Komunikasi Antar-Service (Gateway → Worker)

### 11.1 Analogi dengan Digidoc

Di digidoc, komunikasi ke sistem eksternal dilakukan dengan pola:

```
service → CoreClient interface → CoreClientImpl → lib/httpclient → Core Service (eksternal)
service → AdapterClient interface → AdapterClientImpl → lib/httpclient → Adapter Service (eksternal)
```

Di project kita, Gateway memanggil Worker dengan pola yang **identik**:

```
service → WorkerClient interface → rest/client.go  → lib/httpclient → Worker (REST)
service → WorkerClient interface → grpc/client.go  → gRPC stub      → Worker (gRPC)
```

`internal/worker/` di Gateway adalah analog langsung dari `internal/core/` dan `internal/adapter/` di digidoc. Nama folder mencerminkan nama sistem yang dipanggil, bukan nama abstraksi konsep arsitektur.

### 11.2 Interface WorkerClient

Interface `WorkerClient` didefinisikan di `service/interface.go` — **consumer yang mendefinisikan kontrak yang dibutuhkan**, bukan di dalam package implementasinya. Ini konvensi Go idiomatis: interface didefinisikan di tempat mereka digunakan.

```go
// gateway/internal/service/interface.go
type WorkerClient interface {
    TranscodeVideo(ctx context.Context, input model.VideoPayload) (*model.TranscodeResult, error)
}
```

Dua implementasi yang mengimplementasikan interface ini:

```go
// gateway/internal/worker/rest/client.go
type RestWorkerClient struct {
    httpClient httpclient.Client  // dari lib/httpclient
}
func (c *RestWorkerClient) TranscodeVideo(...) (*model.TranscodeResult, error) { ... }

// gateway/internal/worker/grpc/client.go
type GrpcWorkerClient struct {
    stub pb.VideoServiceClient  // dari gen/
}
func (c *GrpcWorkerClient) TranscodeVideo(...) (*model.TranscodeResult, error) { ... }
```

Service di Gateway tidak pernah tahu apakah yang di-inject adalah REST client atau gRPC client — dependency injection di `cmd/server/main.go` yang menentukan implementasi mana yang dipakai berdasarkan mode yang dijalankan.

### 11.3 lib/httpclient di Gateway

Mengikuti pola digidoc, Gateway membutuhkan `lib/httpclient/` sebagai abstraksi HTTP client yang digunakan oleh `internal/worker/rest/client.go`:

```
gateway/lib/
├── httpclient/
│   ├── httpclient.go        ← interface: Client { Do(ctx, *Request) (*Response, error) }
│   └── httpclient_resty.go  ← implementasi menggunakan go-resty
└── metrics/
    ├── metrics.go
    ├── prometheus.go
    └── nil_metrics.go
```

Worker tidak membutuhkan `lib/httpclient/` karena Worker tidak memanggil service lain.

Struktur `Request` dan `Response` yang dipakai:

```go
// gateway/lib/httpclient/httpclient.go
type Request struct {
    Method  string
    URL     string
    Headers map[string]string
    Body    []byte
}

type Response struct {
    StatusCode int
    Headers    map[string]string
    Body       []byte
}

type Client interface {
    Do(ctx context.Context, req *Request) (*Response, error)
}
```

### 11.4 Perbedaan dari Digidoc

Di digidoc, interface `CoreClient` didefinisikan di dalam package `internal/core/` yang sama dengan implementasinya. Di project kita, `WorkerClient` interface ada di `service/interface.go` agar service layer yang menjadi pemilik kontrak — ini lebih idiomatis di Go.

Selain itu, di digidoc satu `CoreClientImpl` mendukung banyak method (GenerateRegistrationLink, GetUserData, CheckCertificate, dll) karena semua memanggil satu sistem yang sama. Di project kita, `WorkerClient` hanya punya satu method (`TranscodeVideo`) karena memang hanya ada satu operasi yang dikirim ke Worker.

---

## 12. Mekanisme Propagasi Timestamp Antar-Proses

### 12.1 Masalah yang Diselesaikan

Skema lokasi perhitungan mengharuskan setiap pihak yang menghitung selisih waktu memiliki akses ke titik waktu awal yang dicatat oleh pihak lain di proses yang berbeda. Contoh: Worker menghitung (t4-t3), tapi t3 dicatat di proses Gateway — Worker tidak otomatis tahu nilai t3 itu. Diperlukan mekanisme eksplisit untuk mengirim nilai timestamp lintas proses, lewat jalur komunikasi yang sudah ada (REST atau gRPC), bukan lewat Go `context.Context` biasa (yang hanya hidup dalam satu proses dan tidak menyeberang network secara otomatis).

### 12.2 Mekanisme untuk REST: HTTP Header Generic

Setiap HTTP message (request atau response) yang terlibat dalam propagasi ini hanya membawa satu timestamp, sehingga digunakan satu nama header generic: `X-Timestamp`. Tidak perlu nama spesifik per titik (`X-T1-Nano`, dst) karena peran timestamp sudah jelas dari arah dan konteks message itu sendiri.

```
Client → Gateway (request)   : X-Timestamp = t1
Gateway → Worker (request)   : X-Timestamp = t3
Worker → Gateway (response)  : X-Timestamp = t5
Gateway → Client (response)  : X-Timestamp = t7
```

Format nilai: Unix nanosecond (`time.Time.UnixNano()`), dikonversi ke string.

### 12.3 Mekanisme untuk gRPC: Metadata (Header, BUKAN Trailer)

Koreksi penting yang sudah divalidasi: seluruh propagasi timestamp di gRPC menggunakan header metadata, tidak ada satupun yang memakai trailer metadata. Klaim awal yang menyebut t5 dikirim sebagai trailer sudah dikoreksi dan salah — yang benar tetap header metadata.

Ada dua pola pemakaian header metadata yang berbeda secara teknis, tergantung kapan nilai timestamp itu diketahui dalam siklus satu RPC call:

**Pola A — untuk t1 dan t3 (outgoing metadata di sisi pemanggil, sebelum call dibuka):**
```go
md := metadata.Pairs("x-timestamp", timing.EncodeTimestamp(t1))
ctx = metadata.NewOutgoingContext(ctx, md)
stream, err := client.SomeStreamingRPC(ctx)
```
Nilai sudah diketahui sebelum stream dibuka — ini pola default/standar, tidak ada timing khusus yang perlu diperhatikan. Penerima membaca lewat `metadata.FromIncomingContext(stream.Context())` di awal handler, sebelum loop `Recv()` apapun.

**Pola B — untuk t5 dan t7 (header metadata dari SERVER, ditentukan di tengah eksekusi handler):**
Nilai t5/t7 baru diketahui setelah fase menerima selesai dan sebelum fase mengirim dimulai — di tengah satu fungsi handler yang sama (karena RPC kita bidirectional streaming yang dipakai secara berurutan: terima semua dulu, baru kirim semua).
```go
headerMD := metadata.Pairs("x-timestamp", timing.EncodeTimestamp(t5))
stream.SetHeader(headerMD)
// SetHeader HARUS dipanggil SEBELUM Send() pertama, atau akan gagal
for _, output := range result.Outputs {
    stream.Send(&pb.TranscodeChunk{...})  // Send() pertama otomatis mengirim header yang sudah disiapkan
}
```
Constraint resmi: SetHeader gagal jika dipanggil setelah Send() pertama atau setelah handler return.

Di sisi penerima, `stream.Header()` bersifat blocking — menunggu sampai metadata benar-benar tiba, tidak langsung gagal/kosong kalau dipanggil "terlalu cepat". Hanya kembali kosong tanpa error kalau stream berakhir tanpa pernah ada header yang dikirim sama sekali.

```go
headerMD, _ := stream.Header()  // blocking, aman dipanggil kapan saja setelah Recv pertama
t5Str := headerMD.Get("x-timestamp")[0]
```

### 12.4 Perbedaan Go Context vs gRPC Metadata — Jangan Disamakan

`context.Context` murni Go hanya hidup dalam satu proses (RAM), tidak pernah menyeberang network secara otomatis. gRPC metadata secara teknis "ditempelkan" lewat `context.Context` di level API (`metadata.NewOutgoingContext`), tapi yang benar-benar menyeberang network adalah hasil serialisasi oleh gRPC framework sendiri (dikirim sebagai HTTP/2 header di balik layar) — bukan Go context yang "ikut terbang". Dua hal yang terlihat mirip secara API tapi mekanismenya berbeda secara fundamental.

### 12.5 Implikasi ke Interface WorkerClient — Disederhanakan dari Skema Awal

Karena lokasi perhitungan Segmen 2 (t4-t3) sudah dipindah ke WORKER (bukan didekati dari Gateway), Worker tidak perlu melaporkan balik t4 ke Gateway — t4 sepenuhnya selesai diproses lokal di Worker. Worker hanya perlu melaporkan t5 balik ke Gateway (supaya Gateway bisa hitung t6-t5 untuk Segmen 3).

```go
// gateway/internal/service/interface.go — REVISI
type WorkerClient interface {
    // ProcessVideo mengirim payload ke worker beserta t3, menerima hasil beserta t5 dari worker.
    ProcessVideo(ctx context.Context, payload model.VideoPayload, t3 time.Time) (result *model.TranscodeResult, t5 time.Time, err error)
}
```

Worker, di sisi handler-nya sendiri, langsung menghitung (t4-t3) dan mencatatnya ke Prometheus histogram miliknya sendiri — tidak ada nilai yang perlu dikirim balik untuk keperluan itu.

### 12.6 Shared Utility untuk Encode/Decode Timestamp

Disepakati: logika encode/decode timestamp (dipakai identik oleh Gateway dan Worker, untuk REST header maupun gRPC metadata) masuk ke `shared/timing/`, bukan diduplikasi di masing-masing service.

```go
// shared/timing/timing.go
package timing

const HeaderTimestamp = "X-Timestamp" // dipakai sebagai key, baik untuk HTTP header maupun gRPC metadata

func EncodeTimestamp(t time.Time) string {
    return strconv.FormatInt(t.UnixNano(), 10)
}

func DecodeTimestamp(s string) (time.Time, error) {
    nano, err := strconv.ParseInt(s, 10, 64)
    if err != nil {
        return time.Time{}, err
    }
    return time.Unix(0, nano), nil
}
```

### 12.7 Soal Middleware — Hanya Cocok untuk Separuh Mekanisme

Middleware Gin (atau gRPC interceptor) hanya cocok untuk ekstraksi masuk (membaca t1/t3 di awal handler, sebelum business logic apapun jalan) — ini titik yang konsisten "sebelum handler" sehingga bisa digeneralisasi.

Middleware TIDAK cocok untuk injeksi keluar (mencatat dan mengirim t7/t5) karena titik itu terjadi di tengah eksekusi handler (setelah fase terima selesai, sebelum fase kirim dimulai) — bukan titik generik "sebelum/sesudah handler" yang bisa ditangkap middleware. Bagian ini tetap harus berupa pemanggilan fungsi eksplisit langsung di kode handler.

Keputusan belum final soal apakah middleware tetap dipakai untuk separuh ekstraksi-masuk itu — mengingat project ini hanya punya satu RPC utama per service, manfaat abstraksi middleware kecil. Kemungkinan besar: panggil langsung fungsi shared/timing di kode handler tanpa layer middleware tambahan.

### 12.8 Mekanisme Pencatatan Segmen 4 (t8-t7) dari Client

Client adalah proses short-lived (jalan sekali, selesai, exit) — tidak punya server `/metrics` sendiri yang bisa terus di-scrape Prometheus. Tiga alternatif sudah dievaluasi dan ditolak:

- CSV lokal — ditolak oleh user, dianggap tidak cocok dengan pendekatan project ini.
- Prometheus Pushgateway — diteliti dan ditolak karena Pushgateway hanya menyimpan nilai terakhir per kombinasi job/instance (bukan histori). Untuk kebutuhan 30 run per skenario yang masing-masing harus jadi data point terpisah, Pushgateway secara struktural tidak cocok.
- Client jalankan server `/metrics` sendiri — ditolak karena: (1) client harus tetap hidup hanya supaya bisa discrape, bertentangan dengan sifat CLI yang harus selesai-dan-keluar; (2) konflik dengan Skenario 2 (banyak instance CLI bersamaan, masing-masing butuh port berbeda, butuh service discovery dinamis yang kompleks).

Solusi yang dipilih: Client menghitung (t8-t7) secara lokal setelah selesai menyimpan semua file hasil, lalu mengirim nilai itu ke Gateway lewat satu endpoint REST tambahan (contoh: `POST /v1/report-metric`), sebelum Client exit. Gateway menerima laporan ini dan mencatatkannya ke Prometheus histogram miliknya sendiri, dengan label segmen `gateway_to_client`.

### 12.9 Distribusi Final Objek Metrik per Proses

```
GATEWAY /metrics berisi histogram untuk:
  - Segmen 1 (t2-t1, Client→Gateway) — dihitung lokal di Gateway
  - Segmen 3 (t6-t5, Worker→Gateway) — dihitung lokal di Gateway, t5 didapat dari Worker
  - Segmen 4 (t8-t7, Gateway→Client) — dihitung di Client, dilaporkan ke Gateway via endpoint report

WORKER /metrics berisi histogram untuk:
  - Segmen 2 (t4-t3, Gateway→Worker) — dihitung lokal di Worker, t3 didapat dari Gateway
  - CPU dan memori proses Worker sendiri (ProcessCollector, GoCollector)
```

Worker TIDAK perlu melaporkan apapun balik ke Gateway untuk keperluan Segmen 2 — perhitungan itu selesai total di sisi Worker. Worker HANYA mengirim t5 balik ke Gateway (lewat header/metadata response), khusus untuk keperluan Gateway menghitung Segmen 3.

---

## 13. Mekanisme Pengukuran CPU dan Memori per Skenario

### 13.1 Masalah Dasar

CPU (`process_cpu_seconds_total`, kumulatif) dan memori (`process_resident_memory_bytes`, nilai sesaat) secara struktural berbeda dari latency/throughput — mereka bukan kejadian diskrit per video, melainkan nilai yang berjalan terus tanpa mengetahui request mana yang sedang dilayani. Untuk Skenario 2 (concurrent), banyak request berjalan bersamaan dalam satu proses Go yang sama, sehingga atribusi CPU/RAM ke satu request individual **tidak valid secara metodologis** — tidak ada cara membedakan kontribusi CPU request A vs request B saat keduanya tumpang tindih waktu dalam satu proses.

### 13.2 Keputusan: Alternatif C — Metode Berbeda per Tipe Skenario

Disepakati setelah membandingkan beberapa alternatif (rata-rata per skenario saja, per-request via selisih counter, campuran, serta Docker/cgroups stats yang sudah ditolak sebelumnya karena cAdvisor sudah dikeluarkan dari arsitektur):

```
Skenario 1 (sequential, 1080p)         → CPU/RAM dihitung PER REQUEST (30 angka terpisah)
Skenario 3 (sequential, per ukuran)    → CPU/RAM dihitung PER REQUEST (30 angka terpisah per kelompok ukuran)
Skenario 2 (concurrent, per level)     → CPU/RAM dihitung PER BATCH (1 angka per level concurrency)
```

Alasan pembedaan: Skenario 1 dan 3 berjalan sequential (tidak ada tumpang tindih waktu antar-request), sehingga atribusi per-request valid. Skenario 2 sengaja dijalankan bersamaan, sehingga atribusi per-request tidak valid — diukur sebagai satu angka representatif per level beban.

### 13.3 Mekanisme untuk Skenario 1 dan 3 (Per-Request)

CPU dan memori dibaca **langsung di kode Go** pada titik t3, t4, t5, dan saat selesai kirim — bukan menunggu sampel otomatis dari Prometheus. Fase FFmpeg (t4→t5) dikeluarkan total dari perhitungan.

**CPU** (kumulatif, pakai selisih dan dibagi durasi):
```
fase_terima:  (cpu_pada_t4 - cpu_pada_t3) / (t4 - t3)
fase_kirim:   (cpu_pada_selesai_kirim - cpu_pada_t5) / (selesai_kirim - t5)

usage_request = (delta_cpu_terima + delta_cpu_kirim) / (durasi_terima + durasi_kirim)
```

**Memori** (nilai sesaat, rata-rata dari titik-titik fase transfer saja):
```
rata-rata_request = (ram_pada_t3 + ram_pada_t4 + ram_pada_t5 + ram_pada_selesai_kirim) / 4
```

Hasil dari satu request (satu angka CPU, satu angka RAM) dimasukkan ke histogram Prometheus lewat `Observe()`, dengan struktur identik seperti latency — sehingga setelah 30 request, tersedia distribusi yang bisa dihitung rata-rata maupun P50/P95/P99, dengan label tambahan `scenario` (dan `payload_size` khusus Skenario 3).

**Status implementasi: belum ada di kode.** Logika pembacaan CPU/RAM per-request ini belum ditambahkan ke handler manapun (gateway maupun worker). Ini pekerjaan terbuka yang perlu masuk ke scope implementasi berikutnya.

### 13.4 Mekanisme untuk Skenario 2 (Per-Batch)

Tidak ada pembacaan manual di kode untuk skenario ini — dipakai mekanisme scrape otomatis Prometheus (interval 1 detik) yang berjalan terus tanpa peduli batch mana yang sedang aktif, dikombinasikan dengan pencatatan waktu manual oleh peneliti.

**Langkah prosedural (di luar kode, perlu dilakukan manual/lewat script saat pengujian):**
1. Catat jam mulai tepat sebelum batch N-concurrent dijalankan
2. Jalankan batch (N instance CLI bersamaan via `&`, `wait` sampai semua selesai)
3. Catat jam selesai tepat setelah `wait` kembali

**Perhitungan setelah filter rentang waktu di Grafana (PromQL):**
```
CPU    : rate(process_cpu_seconds_total[rentang_waktu_batch])
Memori : avg_over_time(process_resident_memory_bytes[rentang_waktu_batch])
```

`rate()` dipakai untuk CPU karena metriknya kumulatif (counter) — menghitung `(nilai_akhir - nilai_awal) / durasi`. `avg_over_time()` dipakai untuk memori karena metriknya nilai sesaat (gauge) — merata-ratakan semua titik sampel dalam rentang.

Hasil akhir: **satu** angka CPU dan **satu** angka RAM per level concurrency (5, 10, 20, 50), bukan distribusi 30 angka — diulang untuk REST dan gRPC.

**Status implementasi: belum ada.** Belum ada script atau mekanisme pencatatan jam mulai/selesai batch yang terformalisasi.

### 13.5 Rasional Interval Scrape 1 Detik

`scrape_interval: 1s` di `prometheus.yml` bukan nilai baku/minimum dari Prometheus — itu nilai konfigurasi yang dipilih sendiri, sudah tercantum di Section 6.4 sejak awal. Pertimbangan pemilihannya: interval terlalu besar (seperti default industri 15 detik) berisiko menghasilkan terlalu sedikit titik data untuk batch yang berdurasi pendek (hitungan detik); interval terlalu kecil berisiko menambah overhead scrape yang ikut mencemari angka CPU/RAM yang sedang diukur. 1 detik dipilih sebagai titik seimbang untuk durasi batch yang umumnya berkisar beberapa detik sampai puluhan detik di penelitian ini.

### 13.6 Ringkasan Status

| Skenario | Granularitas hasil | Mekanisme pengukuran | Status kode |
|---|---|---|---|
| 1 | 30 angka per protokol | Baca langsung di kode (t3/t4/t5/selesai), exclude FFmpeg | Belum diimplementasikan |
| 3 | 30 angka per kelompok ukuran per protokol | Sama seperti Skenario 1 | Belum diimplementasikan |
| 2 | 1 angka per level concurrency per protokol | Scrape otomatis 1 detik + filter waktu manual di Grafana | Belum diimplementasikan (termasuk mekanisme pencatatan jam batch) |

### 13.7 Catatan Penting: Asimetri Makna CPU/RAM pada Segmen 4 (Gateway→Client)

Client tidak melakukan pembacaan CPU/RAM (`resource.Read()`) sama sekali, karena `resource.Read()` hanya relevan untuk proses server permanen yang punya endpoint `/metrics` (Gateway dan Worker) — Client adalah proses short-lived tanpa instrumentasi resource monitoring (lihat Section 12.8).

Akibatnya, terdapat asimetri makna antara CPU/RAM keempat segmen:

```
Segmen 1 (Client→Gateway)  : CPU/RAM milik GATEWAY — biaya Gateway MENERIMA dari Client
Segmen 2 (Gateway→Worker)  : CPU/RAM milik WORKER  — biaya Worker MENERIMA dari Gateway
Segmen 3 (Worker→Gateway)  : CPU/RAM milik GATEWAY — biaya Gateway MENERIMA dari Worker
Segmen 4 (Gateway→Client)  : CPU/RAM milik GATEWAY — biaya Gateway MENGIRIM ke Client
                              (BUKAN biaya Client menerima — Client tidak diukur)
```

Tiga segmen pertama konsisten mengukur biaya pihak **penerima**. Segmen 4 berbeda — dia mengukur biaya pihak **pengirim** (Gateway), karena pihak penerima sesungguhnya (Client) tidak terinstrumentasi.

**Ini bukan kesalahan implementasi** — ini konsekuensi yang disengaja dari keputusan Section 12.8 (Client tidak menjalankan server `/metrics` sendiri). Tujuan pengukuran CPU/RAM adalah membandingkan REST vs gRPC, bukan membandingkan Gateway vs Client. Perbedaan REST vs gRPC pada Segmen 4 tetap terukur dengan baik dari sisi Gateway, karena perbedaan cara kerja kedua protokol (loop tulis JSON header + binary dengan `Flush()` manual untuk REST, vs loop `stream.Send()` untuk gRPC) memang terjadi di kode Gateway, bukan di kode Client.

**Wajib dicantumkan di laporan (Bab 4/5):** perlu kalimat penjelasan eksplisit bahwa CPU/RAM Segmen Gateway→Client merepresentasikan biaya proses Gateway saat menulis response, bukan biaya proses Client saat menerima — supaya pembaca tidak salah mengira data Segmen 4 sebanding langsung dengan tiga segmen lain dari sudut pandang yang sama (pihak penerima).

---

## 14. Revisi Jumlah Skenario: dari Tiga Menjadi Dua

### 14.1 Alasan Penggabungan

Skenario 1 (File Tunggal Berukuran Besar) dan Skenario 3 (Variasi Ukuran Payload) ditemukan **identik secara mekanis** setelah keputusan terbaru menetapkan resolusi tetap di kedua skenario tersebut. Skenario 1 pada dasarnya hanya menjadi satu titik tambahan pada deret ukuran yang sudah dicakup Skenario 3, tanpa perbedaan metode pengukuran, metode perhitungan persentil, atau cara eksekusi (keduanya sequential, per-request).

Berdasarkan prinsip variabel terkontrol (hanya satu variabel yang boleh diubah per skenario), keduanya digabungkan menjadi satu skenario.

**Catatan administratif:** keputusan ini mengubah struktur metodologi yang mungkin sudah dideskripsikan di proposal yang sudah melalui Desk Evaluation 1. Perlu dikonfirmasi ke dosen pembimbing sebelum dianggap final untuk laporan TA.

### 14.2 Struktur Dua Skenario yang Baru

**Skenario A — Variasi Ukuran Payload (gabungan Skenario 1 dan 3 sebelumnya)**
```
Resolusi    : 1080p (tetap, untuk seluruh titik ukuran)
Ukuran file : 10MB, 50MB, 100MB, 250MB, 500MB
Mekanisme   : sequential, per-request
Tujuan      : titik 500MB merepresentasikan kondisi beban berat (menggantikan
              tujuan Skenario 1 lama); seluruh titik bersama-sama menemukan
              titik perpotongan performa REST vs gRPC seiring ukuran file
              bertambah (tujuan Skenario 3 lama)
```

**Skenario B — Banyak Request Bersamaan (sebelumnya Skenario 2, tidak berubah)**
```
Resolusi         : 720p (tetap)
Ukuran file      : 5MB (tetap)
Level concurrency: 5, 10, 20, 50
Mekanisme        : 30 batch per level, latency/throughput per-request,
                    CPU/RAM per-batch (rata-rata dari rata-rata)
```

### 14.3 Expected Output per Skenario (Final)

**Skenario A, untuk setiap titik ukuran file:**
```
Latency    : P50, P95, P99 dari 30 data (REST), P50, P95, P99 dari 30 data (gRPC)
Throughput : P50, P95, P99 dari 30 data (REST), P50, P95, P99 dari 30 data (gRPC)
CPU        : P50, P95, P99 dari 30 data (REST), P50, P95, P99 dari 30 data (gRPC)
RAM        : P50, P95, P99 dari 30 data (REST), P50, P95, P99 dari 30 data (gRPC)
```

**Skenario B, untuk setiap level concurrency:**
```
Latency    : P50, P95, P99 dari (level x 30) data, per protokol
Throughput : P50, P95, P99 dari (level x 30) data, per protokol
CPU        : satu angka rata-rata akhir (dari rata-rata 30 batch), per protokol
RAM        : satu angka rata-rata akhir (dari rata-rata 30 batch), per protokol
```

### 14.4 Mekanisme Pengumpulan Data — Ringkasan Final

**Skenario A — sepenuhnya otomatis (full by system):**
1. Client kirim request, t1 sampai t8 dicatat dan dipropagasi sesuai mekanisme Section 12
2. CPU/RAM per-request dibaca langsung via `resource.Read()` di titik t1/t2/t7/selesai-kirim (Gateway) dan t3/t4/t5/selesai-kirim (Worker)
3. Seluruh angka (latency, throughput, CPU, RAM) dimasukkan ke histogram via `Observe()`
4. Prometheus scrape `/metrics` setiap 1 detik, menyimpan histori histogram tersebut
5. Grafana hitung `histogram_quantile()` untuk P50/P95/P99, langsung dari histogram, tanpa filter rentang waktu manual

**Skenario B — ada satu langkah manual:**
1. **(Manual/script)** Catat jam mulai dan jam selesai untuk setiap dari 30 batch per level concurrency
2. **(Otomatis)** Latency/throughput tiap request tetap dicatat dan diproses sama seperti Skenario A — tidak ada langkah manual untuk dua metrik ini
3. **(Otomatis)** Prometheus tetap scrape CPU/RAM mentah setiap 1 detik, terus-menerus, tanpa peduli batch
4. **(Manual)** Untuk setiap batch, masukkan jam mulai-selesai yang sudah dicatat ke query Grafana (`rate()` untuk CPU, `avg_over_time()` untuk RAM), hasilkan 1 angka per batch
5. **(Manual/dihitung ulang)** Rata-ratakan 30 angka per-batch tersebut menjadi satu angka akhir per level per protokol

Catatan: langkah manual pada Skenario B berpotensi diotomatisasi lewat script setelah Client CLI dan mekanisme pencatatan batch dibuat (lihat Section 15, Fase 8).

---

## 15. Pembagian Fase Implementasi Kode (Sejak Commit "feat: implement timestamp propagation")

Status seluruh fase di bawah ini: **BELUM ADA SATU PUN YANG DIKERJAKAN**, terhitung sejak commit terakhir. Daftar ini menjadi acuan checklist untuk pelacakan progres implementasi berikutnya.

### Fase 1 — Infrastruktur CPU/RAM: Shared dan Worker
**Status: ⬜ Belum dikerjakan**
```
shared/resource/resource.go                          BARU
worker/lib/metrics/metrics.go                         EDIT
worker/lib/metrics/prometheus.go                      EDIT
worker/lib/metrics/nil_metrics.go                     EDIT
worker/internal/handler/rest/video_handler.go         EDIT
worker/internal/handler/grpc/video_handler.go         EDIT
```
Tujuan: Worker bisa membaca CPU/RAM secara manual di titik t3/t4/t5/selesai-kirim, menghitung satu angka per-request, dan mencatatkannya ke histogram baru.

### Fase 2 — Infrastruktur CPU/RAM: Gateway (mirror Fase 1)
**Status: ⬜ Belum dikerjakan**
```
gateway/lib/metrics/metrics.go                        EDIT
gateway/lib/metrics/prometheus.go                     EDIT
gateway/lib/metrics/nil_metrics.go                    EDIT
gateway/internal/handler/rest/video_handler.go        EDIT
gateway/internal/handler/grpc/video_handler.go        EDIT
```
Tujuan: sama seperti Fase 1, tapi untuk Gateway, di titik t1/t2/t7/selesai-kirim.

### Fase 3 — Endpoint Report Metric di Gateway
**Status: ⬜ Belum dikerjakan**
```
gateway/internal/handler/rest/metric_handler.go       BARU
gateway/internal/handler/rest/routes.go                EDIT
gateway/internal/model/video_dto.go                    EDIT
```
Tujuan: menyediakan `POST /v1/report-metric` agar Client bisa melaporkan hasil hitungan Segmen 4 (t8-t7), karena Client tidak punya `/metrics` sendiri (Section 12.8).

### Fase 4 — Client CLI: Scaffolding, Config, Model, Constant
**Status: ⬜ Belum dikerjakan**
```
client/main.go                                         BARU
client/config/config.go                                BARU
client/internal/constant/protocol.go                    BARU
client/internal/constant/default.go                     BARU
client/internal/model/video.go                          BARU
```
Tujuan: fondasi dasar Client sebelum logika transport dan command ditambahkan.

### Fase 5 — Client CLI: Transport Layer (REST dan gRPC)
**Status: ⬜ Belum dikerjakan**
```
client/internal/transport/interface.go                  BARU
client/internal/transport/rest/client.go                 BARU
client/internal/transport/grpc/client.go                  BARU
```
Tujuan: Client bisa kirim video dan terima hasil dari Gateway, lewat REST maupun gRPC, mengikuti mekanisme propagasi t1/t7 sesuai Section 12.

### Fase 6 — Client CLI: Cobra Command dan Orkestrasi
**Status: ⬜ Belum dikerjakan**
```
client/cmd/root.go                                       BARU
client/cmd/transcode.go                                   BARU
```
Tujuan: flag `--protocol`, `--input`, `--output`; orkestrasi penuh t1 → kirim → terima → t8 → simpan file → hitung t8-t7 → kirim ke endpoint Fase 3.

### Fase 7 — Propagasi Label scenario dan payload_size
**Status: ⬜ Belum dikerjakan — bergantung pada Fase 1-6 selesai**
```
client/cmd/transcode.go                                   EDIT — tambah flag --scenario
gateway/internal/handler/rest/video_handler.go             EDIT — baca header label, teruskan
gateway/internal/handler/grpc/video_handler.go              EDIT
worker/internal/handler/rest/video_handler.go                EDIT — baca label diteruskan Gateway
worker/internal/handler/grpc/video_handler.go                 EDIT
(seluruh prometheus.go di Gateway dan Worker)                  EDIT — tambah label ke semua histogram
```
Catatan: `concurrency_level` TIDAK relevan untuk CPU/RAM (lihat Section 13), hanya relevan untuk latency/throughput.

### Revisi Fase 7 — Propagasi Label scenario, payload_size, concurrency_level

**Status: ⬜ Belum dikerjakan — revisi dari estimasi awal di atas, scope lebih besar dari perkiraan semula**

Estimasi awal Fase 7 (6 file) tidak memperhitungkan dampak berantai ke call chain yang sudah stabil sejak Fase 2 (`service.Transcode()` → `workerClient.ProcessVideo()`). Revisi ini menggantikan estimasi awal.

#### Keputusan Desain

1. **Tiga label baru dibungkus dalam satu struct**, bukan ditambah sebagai parameter satu-satu ke empat method metrics:
```go
type Labels struct {
	Scenario         string
	PayloadSize      string
	ConcurrencyLevel string // string kosong kalau tidak relevan
}
```

2. **Jumlah label berbeda antara latency/throughput dan CPU/RAM**, sesuai keputusan Section 13:
```
latency, throughput  : label = [segment, protocol, scenario, payload_size, concurrency_level]
cpu, memory          : label = [segment, protocol, scenario, payload_size]   (TANPA concurrency_level)
```
Alasan: `concurrency_level` hanya relevan untuk Skenario B, dan CPU/RAM di Skenario B tidak pernah lewat `Observe()` per-request (lihat Section 13.4) — jadi label itu tidak pernah punya tempat untuk ditempelkan di histogram CPU/RAM.

3. **`payload_size` dihitung otomatis dari ukuran file** (`len(data)`) di Client, dibulatkan jadi label seperti `"10mb"`, `"100mb"` — tidak perlu flag manual.

4. **`scenario` dan `concurrency_level` dikirim lewat flag CLI baru**: `--scenario` dan `--concurrency-level`.

#### Mekanisme Propagasi

Label-label ini mengalir lewat rantai pemanggilan yang sama dengan mekanisme `t3`/`t5` (Section 12), dikirim sebagai HTTP header tambahan (REST) atau gRPC metadata tambahan, bukan lewat `X-Timestamp` yang sudah ada — pakai header/metadata terpisah (`X-Scenario`, `X-Payload-Size`, `X-Concurrency-Level`).

#### Daftar Lengkap File (19 file, dibagi 6 sub-fase)

**Sub-fase 7a — shared/label dan Client (6 file)**
```
shared/label/label.go                                  BARU
client/internal/model/video.go                          EDIT
client/internal/transport/interface.go                  EDIT
client/internal/transport/rest/client.go                 EDIT
client/internal/transport/grpc/client.go                  EDIT
client/cmd/transcode.go                                    EDIT
```

**Sub-fase 7b — Gateway: infrastruktur metrics (3 file)**
```
gateway/lib/metrics/metrics.go                              EDIT
gateway/lib/metrics/prometheus.go                             EDIT
gateway/lib/metrics/nil_metrics.go                             EDIT
```

**Sub-fase 7c — Gateway: service dan worker-client (4 file)**
```
gateway/internal/service/interface.go                           EDIT
gateway/internal/service/video_service.go                        EDIT
gateway/internal/worker/rest/client.go                             EDIT
gateway/internal/worker/grpc/client.go                              EDIT
```

**Sub-fase 7d — Gateway: handler (2 file)**
```
gateway/internal/handler/rest/video_handler.go                       EDIT
gateway/internal/handler/grpc/video_handler.go                        EDIT
```

**Sub-fase 7e — Worker: infrastruktur metrics (3 file)**
```
worker/lib/metrics/metrics.go                                           EDIT
worker/lib/metrics/prometheus.go                                         EDIT
worker/lib/metrics/nil_metrics.go                                         EDIT
```

**Sub-fase 7f — Worker: handler (2 file)**
```
worker/internal/handler/rest/video_handler.go                             EDIT
worker/internal/handler/grpc/video_handler.go                              EDIT
```

Urutan eksekusi: 7a → 7b → 7c → 7d → 7e → 7f (mengikuti arah aliran data dari Client sampai Worker).

### Fase 8 — Script Pencatatan Waktu Batch untuk Skenario B
**Status: ⬜ Belum dikerjakan — bukan kode Go, berupa script bash terpisah**
```
scripts/run_concurrent_batch.sh                            BARU (di luar struktur Go module)
```
Tujuan: jalankan N instance Client bersamaan, catat jam mulai-selesai tiap batch ke file log, diulang 30 kali per level concurrency, menggantikan langkah manual yang dijelaskan di Section 14.4.

### Fase 9 — Setup Prometheus
**Status: ✅ Selesai.** Awalnya dijalankan sebagai binary lokal di WSL (`prometheus.yml`, scrape `localhost:2112`/`localhost:2113`, verifikasi manual lewat `localhost:9090`). Per Fase 11 (2026-07-06), Prometheus dipindah masuk ke `docker-compose.yml` sebagai service `prometheus` (image `prom/prometheus:v2.53.0`), pakai config baru `prometheus.docker.yml` dengan target berbasis nama service Docker (`gateway:2112`, `worker:2113`). `prometheus.yml` versi lokal lama dibiarkan tidak dihapus untuk alur `go run` manual kalau sewaktu-waktu masih dipakai.

### Fase 10 — Setup Grafana
**Status: ✅ Selesai.** Sama seperti Fase 9, awalnya instalasi lokal manual di WSL (port 3000, datasource Prometheus di-setup lewat UI). Per Fase 11, Grafana dipindah ke `docker-compose.yml` sebagai service `grafana` (image `grafana/grafana:11.6.0`), dengan datasource Prometheus di-auto-provision lewat `grafana/provisioning/datasources/prometheus.yml` (tidak perlu setup ulang manual lewat UI tiap kali container dibuat ulang).

### Fase 11 — Dockerisasi dan Round-Robin 2 Worker (Ditunda Terakhir)
**Status: ✅ Selesai sebagian (2026-07-06) — Dockerisasi Gateway+Worker+Prometheus+Grafana SELESAI, round-robin 2 Worker MASIH TERTUNDA.**
```
gateway/Dockerfile                                       BARU
worker/Dockerfile                                        BARU
docker-compose.yml                                        BARU
.dockerignore                                              BARU
prometheus.docker.yml                                      BARU
grafana/provisioning/datasources/prometheus.yml             BARU
gateway/cmd/server/main.go                                 BELUM diubah — round-robin antara Worker1 dan Worker2 masih tertunda
```
Fase ini akhirnya dipecah jadi dua tahap saat eksekusi (lihat Section 8 untuk detail lengkap): Tahap 1 (Dockerisasi dengan 1 Worker, sudah selesai dan diverifikasi end-to-end) dan Tahap 2 (tambah `worker-2` + logic round-robin di Gateway, memperbaiki temuan F1, belum dikerjakan — ditunda ke sesi terpisah setelah Tahap 1 stabil dipakai testing resmi). Alasan Dockerisasi dikerjakan lebih dulu daripada rencana awal ("paling akhir" setelah semua testing lokal selesai): ditemukan saat mulai testing resmi bahwa loopback `localhost` menghasilkan throughput yang tidak representatif kondisi microservices nyata (lihat Section 16.6), sehingga Dockerisasi dipercepat jadi prasyarat sebelum pengambilan data resmi Skenario A/B, bukan lagi murni tahap "nice to have" di akhir.

### Catatan Urutan yang Diperbarui

```
Fase 1 → Fase 2 → Fase 3 → Fase 4 → Fase 5 → Fase 6 (SELESAI)
   ↓
Fase 7 (label scenario/payload_size — WAJIB sebelum Fase 8 untuk Skenario A,
        karena tanpa label ini data dari berbagai ukuran file akan tercampur
        dalam satu histogram, merusak tujuan utama Skenario A)
   ↓
Fase 8 (script batch — valid dijalankan untuk Skenario B tanpa Fase 7,
        karena Skenario B sudah punya pencatatan jam batch yang bisa
        dipakai sebagai filter rentang waktu, tidak bergantung label)
   ↓
Fase 9 → Fase 10 (setup Prometheus dan Grafana, bisa dikerjakan paralel
        dengan Fase 7-8, tidak saling bergantung)
   ↓
Fase 11 (Dockerisasi, ditunda paling akhir)
```

### Catatan Tambahan untuk Fase 8: Penamaan File Output Harus Unik per Batch

**Temuan:** `saveResults()` di `client/cmd/transcode.go` menulis file dengan pola nama `<nama_asli>_<resolusi>.mp4`, memakai `os.WriteFile` yang **selalu menimpa** file yang sudah ada di path yang sama tanpa peringatan.

**Untuk Skenario A (sequential):** Tidak masalah — file video hasil transcoding cuma bukti proses berhasil, bukan data yang dianalisis. Data metrik (latency, throughput, CPU, RAM) sudah tersimpan terpisah di Prometheus per-request, tidak bergantung pada file fisik di disk.

**Untuk Skenario B (concurrent):** Berpotensi masalah nyata — kalau N instance Client dijalankan bersamaan dengan input dan output folder yang sama, semuanya menulis ke nama file yang sama persis di waktu yang sama, berpotensi terjadi race condition (saling menimpa file yang sedang ditulis proses lain, menghasilkan file korup).

**Keputusan:** Tambahkan nomor batch ke path output saat Fase 8 (script `run_concurrent_batch.sh`) dikerjakan. Output folder untuk tiap instance dibedakan berdasarkan nomor batch, misalnya:
```
./output/batch-1/
./output/batch-2/
...
./output/batch-30/
```
Implementasinya kemungkinan lewat flag `--output` yang sudah ada di Client, di-generate otomatis oleh script bash saat memanggil tiap instance Client (bukan perubahan kode Go baru), atau lewat flag tambahan `--batch-id` kalau diperlukan kontrol lebih eksplisit dari sisi Client sendiri. Keputusan implementasi detail ditentukan saat eksekusi Fase 8.

### Koreksi Catatan Fase 8: Penamaan File Output per Batch (Menggantikan Catatan Sebelumnya)

**Definisi "batch":** satu level concurrency (misal level 20) diulang 30 kali sesuai desain Skenario B. Setiap satu kali pengulangan itu adalah satu batch. Di dalam satu batch, ada N instance Client (N = level concurrency) yang berjalan bersamaan — sehingga dibutuhkan DUA angka unik per file: nomor batch (1-30) DAN nomor instance di dalam batch itu (1-N), bukan cuma satu angka.

**Pendekatan yang DITOLAK:** Client (kode Go) mendeteksi otomatis nomor batch terakhir dengan membaca isi folder output. Ini DITOLAK karena kalau N instance dijalankan bersamaan, semuanya bisa membaca kondisi folder yang sama sebelum ada yang sempat menulis update — menciptakan race condition baru, bukan solusi.

**Solusi final:** Nomor batch dan nomor instance dikontrol oleh SCRIPT BASH (Fase 8), bukan oleh Client. Script bash sudah otomatis tahu sedang di batch/instance keberapa karena itu murni angka pencacah dari loop bash itu sendiri.

```bash
for batch in $(seq 1 30); do
    for instance in $(seq 1 $CONCURRENCY_LEVEL); do
        client transcode \
            --protocol rest \
            --input video.mp4 \
            --output "./output/level-${CONCURRENCY_LEVEL}/batch-${batch}/instance-${instance}" \
            --scenario scenario_b \
            --concurrency-level "${CONCURRENCY_LEVEL}" &
    done
    wait
done
```

**Implikasi penting: TIDAK ADA perubahan kode Go yang dibutuhkan.** Flag `--output` yang sudah ada sejak Fase 6 sudah cukup fleksibel menerima path apapun, termasuk path dengan nomor batch dan instance di dalamnya. Yang berubah hanya cara script bash (Fase 8) memanggil Client — bukan implementasi `saveResults()` atau bagian kode Go manapun.

## 16. Prosedur Pengujian dan Pencatatan Data (Skenario A dan B)

Section ini melengkapi Section 14 (Revisi Jumlah Skenario) dengan prosedur operasional konkret: kapan data dibaca, kapan histogram direset, dan disiplin pencatatan jam manual untuk Skenario B. Disusun setelah Prometheus berhasil berjalan lokal (Fase 9).

### 16.1 Prinsip Umum — Data Prometheus Tidak Perlu Disimpan Permanen

Kebutuhan penelitian hanya angka P50/P95/P99 (latency, throughput, CPU, RAM) yang dicatat ke laporan TA, bukan histori data yang harus tetap hidup selamanya di Prometheus. Histogram di Gateway/Worker bersifat in-memory per proses (`prometheus.NewRegistry()` baru setiap proses start) — restart proses menghapus seluruh histogram, tidak ada persistensi.

Karena itu, histogram boleh direset (lewat restart proses Gateway/Worker) kapan saja, dengan satu syarat mutlak:

> **Angka harus dibaca dan dicatat ke laporan DULU, baru proses Gateway/Worker direstart.** Urutan terbalik (restart dulu baru membaca) menghapus data yang belum sempat dicatat.

### 16.2 Prosedur Skenario A (Variasi Ukuran Payload)

Seluruh metrik (latency, throughput, CPU, RAM) direkam per-request lewat `Observe()`, berlabel `payload_size`. Karena tiap titik ukuran sudah otomatis terpisah sebagai time series tersendiri lewat label ini, restart antar titik ukuran tidak wajib secara teknis untuk isolasi data — sifatnya murni soal disiplin sesi dan kebersihan canvas pembacaan.

Dua pendekatan berikut sama-sama valid, selama prinsip 16.1 dipegang:

**Opsi A1 — per titik ukuran:**
1. (Opsional) Restart Gateway + Worker sebelum mulai titik ukuran ini.
2. Jalankan 30 run `client transcode --protocol <rest|grpc> --input <file> --scenario scenario_a`.
3. Baca P50/P95/P99 (latency, throughput, CPU, RAM) di Grafana, filter label `payload_size` + `protocol`.
4. Catat angkanya ke laporan.
5. Restart, lanjut ke titik ukuran berikutnya.

**Opsi A2 — semua titik ukuran dulu:**
1. Restart sekali di awal sesi.
2. Jalankan seluruh titik ukuran (10MB → 50MB → 100MB → 250MB → 500MB) berurutan, tanpa restart di antaranya.
3. Di akhir, baca seluruh titik ukuran sekaligus (masing-masing difilter labelnya sendiri), catat semua ke laporan.
4. Baru restart.

Trade-off: Opsi A2 lebih ringkas, tapi blast radius kegagalannya lebih besar — kalau crash terjadi sebelum pembacaan akhir, seluruh titik ukuran yang sudah selesai tapi belum dibaca harus diulang dari awal.

### 16.3 Prosedur Skenario B (Concurrent Users)

Berbeda dari Skenario A, Skenario B punya dua metrik dengan mekanisme pengukuran yang sama sekali berbeda, sehingga perlu ditangani terpisah.

**(a) Latency & Throughput**

Tetap direkam per-request lewat `Observe()`, ditambah label `concurrency_level` (terisi otomatis dari flag `--concurrency-level` tiap instance client). Karena ini juga sekadar label tambahan, logikanya identik dengan Section 16.2 — Opsi A1/A2 berlaku sama persis di sini (per level vs semua level), dengan trade-off yang sama.

**(b) CPU & RAM**

TIDAK direkam per-request — sesuai Section 13.1, atribusi CPU/RAM ke satu request individual tidak valid kalau banyak request overlap waktu dalam satu proses. Sumber datanya adalah metrik mentah `process_cpu_seconds_total` (counter) dan `process_resident_memory_bytes` (gauge) yang di-scrape otomatis tiap 1 detik, tanpa label `concurrency_level` sama sekali — isolasi antar batch dilakukan murni lewat rentang waktu yang dicatat manual.

Granularitas pencatatan jam adalah **per batch**, bukan per level — satu level berisi 30 batch, dan tiap batch butuh jam mulai-selesai sendiri (bukan satu rentang besar mencakup 30 batch sekaligus), supaya `rate()` tidak terdilusi oleh jeda istirahat antar batch. Hasil akhir per level adalah rata-rata dari 30 angka per-batch, sesuai Section 14.3.

Prosedur per level concurrency:
1. (Opsional) Restart Gateway + Worker sebelum mulai level ini.
2. Untuk tiap 30 batch dalam level ini:
   - Catat jam mulai.
   - Jalankan N instance client bersamaan (`&` + `wait`), masing-masing dengan `--concurrency-level <N> --scenario scenario_b`.
   - Catat jam selesai.
3. Setelah 30 batch selesai: baca P50/P95/P99 latency/throughput dari Grafana (filter `concurrency_level` + `protocol`), catat ke laporan.
4. Untuk CPU/RAM: hitung `rate(process_cpu_seconds_total[rentang_batch])` dan `avg_over_time(process_resident_memory_bytes[rentang_batch])` untuk masing-masing dari 30 rentang waktu yang sudah dicatat, lalu rata-ratakan jadi satu angka CPU dan satu angka RAM final untuk level ini. Langkah ini boleh dilakukan kapan saja setelah testing selesai — data CPU/RAM tidak hilang akibat restart yang terjadi di antara batch, karena tersimpan di histori Prometheus pada timestamp aslinya, bukan dibaca sebagai nilai instan "sekarang".
5. Catat semua angka (latency, throughput, CPU, RAM) ke laporan. Baru restart, lanjut ke level berikutnya.

**Batasan penting:** restart TIDAK BOLEH terjadi di tengah satu batch yang sedang berjalan — counter `process_cpu_seconds_total` akan ikut reset di tengah rentang waktu batch itu, merusak perhitungan `rate()` untuk batch tersebut. Restart di sela antar batch (setelah satu batch selesai, sebelum batch berikutnya mulai) aman.

### 16.4 Ringkasan Perbedaan Mekanisme

| | Latency & Throughput | CPU & RAM (khusus Skenario B) |
|---|---|---|
| Granularitas | Per-request, otomatis | Per-batch, manual |
| Sumber isolasi data | Label (`payload_size` / `concurrency_level`) | Rentang waktu manual per batch |
| Perlu catat jam? | Tidak | Ya, tiap batch (30×/level) |
| Cara baca | `histogram_quantile()`, instan, tanpa filter waktu | `rate()` / `avg_over_time()`, dibatasi rentang waktu, lalu dirata-rata manual |
| Sensitif terhadap restart? | Ya — wajib dibaca sebelum restart | Tidak terlalu — data tetap valid selama rentang waktunya tercatat dan tidak ada restart di tengah batch |

### 16.5 Prosedur Lifecycle Docker (Pasca Fase 11)

Section ini melengkapi 16.1-16.4 setelah Prometheus dan Grafana pindah dari proses lokal manual di WSL menjadi service dalam `docker-compose.yml` (Section 8). Aturan lama "restart semua 4 komponen (Gateway, Worker, Prometheus, Grafana) antar titik ukuran" perlu disesuaikan, karena keempatnya sekarang punya karakter penyimpanan data yang berbeda.

**Alasan penyesuaian:** Histogram Gateway/Worker tetap bersifat in-memory per proses seperti sebelumnya (restart = counter kumulatif kembali ke nol, sesuai 16.1). Namun Prometheus sekarang menyimpan data di named volume Docker (`prometheus-data`) yang bertahan lintas restart container — merestart container Prometheus TIDAK menghapus data lama di dalamnya (beda karakter dari histogram Gateway/Worker), kecuali volume itu sendiri yang dihapus (`docker compose down -v`). Grafana juga sama, hanya UI query di atas Prometheus, restart tidak mempengaruhi data.

**Prinsip baru:** Prometheus dan Grafana dibiarkan jalan terus-menerus dari awal sampai akhir seluruh sesi pengujian (bahkan bisa mencakup Skenario A dan B sekaligus). Yang di-restart antar titik ukuran hanya Gateway dan Worker — itu pun tetap opsional sesuai prinsip 16.2 (isolasi sudah terjamin lewat label).

```bash
# Sekali di awal sesi hari itu
docker compose up -d
docker compose ps                     # pastikan 4 service running
curl localhost:9090/api/v1/targets    # pastikan target gateway & worker UP

# Antar titik ukuran (Skenario A: payload; Skenario B latency/throughput: concurrency level)
# — HANYA restart gateway+worker, JANGAN restart prometheus/grafana
docker compose restart gateway worker

# Akhir sesi/hari — container berhenti, volume (data Prometheus/Grafana) TETAP ada
docker compose down
```

**Yang wajib dihindari:** menjalankan `docker compose restart` tanpa argumen service — itu me-restart keempat service sekaligus, termasuk Prometheus/Grafana yang seharusnya tetap hidup. Restart blanket seperti ini tidak mencapai apa pun secara teknis (data Prometheus tidak terhapus cuma karena container-nya restart) dan hanya menambah overhead (gap scraping, Grafana sempat disconnect).

**Khusus Skenario B bagian CPU/RAM (16.3(b)):** batasan "restart tidak boleh terjadi di tengah satu batch yang sedang berjalan" (dari 16.3) tetap berlaku sepenuhnya untuk restart `gateway worker`. Tambahan aturan baru: jangan pernah pakai `docker compose down -v` (flag `-v` menghapus volume, termasuk histori Prometheus yang jadi sumber `rate()`/`avg_over_time()` untuk seluruh batch) sebelum semua angka CPU/RAM dari seluruh batch di seluruh level sudah selesai dibaca dan dicatat ke laporan. `docker compose down` biasa (tanpa `-v`) aman dipakai kapan saja karena volume tetap ada.

Ringkasan command per situasi:

| Situasi | Command | Kena Prometheus/Grafana? |
|---|---|---|
| Mulai sesi hari itu | `docker compose up -d` | Ya, sekali di awal |
| Antar payload (Skenario A) | `docker compose restart gateway worker` | Tidak |
| Antar concurrency level (Skenario B, latency/throughput) | `docker compose restart gateway worker` | Tidak |
| Antar batch (Skenario B, CPU/RAM) — HANYA di sela batch, tidak di tengah batch | `docker compose restart gateway worker` | Tidak |
| Akhir sesi/hari | `docker compose down` (tanpa `-v`) | Container berhenti, volume tetap ada |
| Ganti versi image / reset total | `docker compose down -v` | Volume ikut terhapus — hindari sampai semua data sudah dicatat |

### 16.6 Catatan untuk Fase 8

240 kali pencatatan jam manual (30 batch × 4 level × 2 protokol) untuk CPU/RAM Skenario B tidak realistis dikerjakan tangan satu-satu — ini alasan teknis tambahan kenapa Fase 8 (`scripts/run_concurrent_batch.sh`) penting sebelum pengambilan data resmi Skenario B, meski untuk verifikasi mekanisme di Fase 9 cukup dilakukan manual dalam skala kecil (1 batch, beberapa instance saja).
