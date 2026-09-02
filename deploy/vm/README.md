# Deploy BridgeYok pada satu VM

Stack ini menjalankan satu API Go authoritative, satu web Next.js, dan Caddy sebagai
TLS reverse proxy. Targetnya satu instance untuk closed beta; jangan menaikkan jumlah
replica API tanpa desain sticky session dan koordinasi state baru.

## Prasyarat

- VM Linux dengan Docker Engine dan Docker Compose v2.
- DNS `A`/`AAAA` untuk domain web dan API mengarah ke IP VM.
- Port TCP 80/443 dan UDP 443 terbuka. Port API dan web tidak dipublikasikan.
- PostgreSQL/Supabase session-pooler dapat dijangkau dari VM.

## Konfigurasi

Jalankan dari root repository:

```bash
cp deploy/vm/vm.env.example .env.vm
```

Isi `APP_DOMAIN`, `API_DOMAIN`, `DATABASE_URL`, dan `AUTH_SECRET`. Buat secret minimal
32 karakter, misalnya dengan `openssl rand -base64 48`. File `.env.vm` di-ignore Git.

Validasi tanpa memulai container:

```bash
docker compose --env-file .env.vm -f deploy/vm/compose.yaml config --quiet
```

## Deploy

```bash
docker compose --env-file .env.vm -f deploy/vm/compose.yaml up --detach --build
docker compose --env-file .env.vm -f deploy/vm/compose.yaml ps
```

API menjalankan migrasi Goose `up` sebelum menerima traffic. Caddy memperoleh dan
memperbarui sertifikat TLS secara otomatis. WebSocket `/v1/ws` diteruskan oleh Caddy
ke port API yang sama dengan HTTP, sementara heartbeat aplikasi mendeteksi koneksi
mati setiap 20 detik.

Periksa deployment:

```bash
curl --fail --show-error "https://api.example.com/health/live"
curl --fail --show-error "https://api.example.com/health/ready"
docker compose --env-file .env.vm -f deploy/vm/compose.yaml logs --tail 100 api edge
```

Ganti `api.example.com` dengan `API_DOMAIN`. Uji create/join dan reconnect dari browser
sebelum membuka akses beta.

## Update dan rollback

Set `RELEASE_ID` ke identifier release yang unik, lalu build dan start kembali:

```bash
docker compose --env-file .env.vm -f deploy/vm/compose.yaml build api web
docker compose --env-file .env.vm -f deploy/vm/compose.yaml up --detach
```

Deploy single-instance memutus koneksi ketika API diganti. API mendapat waktu drain
30 detik dan klien melakukan resume dari state PostgreSQL sesudah reconnect. Untuk
rollback, kembalikan `RELEASE_ID` atau image release sebelumnya lalu jalankan `up
--detach` lagi. Jangan menjalankan migration `down` saat rollback aplikasi.

## Operasional

```bash
docker compose --env-file .env.vm -f deploy/vm/compose.yaml logs --follow api edge
docker compose --env-file .env.vm -f deploy/vm/compose.yaml restart api
docker compose --env-file .env.vm -f deploy/vm/compose.yaml down
```

Volume `caddy-data` menyimpan sertifikat. Jangan gunakan `down --volumes` pada operasi
normal. Compose menerapkan restart otomatis, health check, log rotation, bounded WS
queues, connection limits, ping/pong, dan graceful shutdown. Stabilitas VM tetap
memerlukan monitoring resource, ruang disk, dan konektivitas database.
