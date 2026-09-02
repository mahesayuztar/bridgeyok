# BridgeYok — Product & Engineering Implementation Plan

> Status: Phase 0–2 selesai; Phase 3 berjalan; Objective GUX UX-01–UX-13 PASS serta tetap menjadi gate untuk ENG-01–03
> Disusun: 29 Agustus 2026
> Refactor scope: 30 Agustus 2026
> Gameplay UX reliability objective: 1 September 2026
> Target: aplikasi online duplicate bridge terfokus untuk closed beta kecil
> Pemilik dokumen: product/engineering
> Aturan perubahan: keputusan yang mengubah protokol, aturan game, data, keamanan, atau biaya harus dicatat sebagai ADR

---

## 1. Ringkasan keputusan

BridgeYok dibangun sebagai **modular monolith**: satu web app Next.js, satu API/realtime service Go, satu PostgreSQL, dan DDS di belakang boundary analysis. Bentuk ini sengaja dioptimalkan untuk server bridge kecil dan stabil, bukan platform komunitas, SaaS, atau layanan internet-scale.

Keputusan utama:

1. **Server selalu authoritative.** Browser hanya mengirim intent/command dan boleh memproyeksikan aksi lokal yang diketahui legal secara optimistik. Browser tidak boleh mengocok kartu, menjadi validator final legality, menghitung pemenang trick, atau menghitung skor authoritative.
2. **PostgreSQL adalah source of truth.** State aktif di RAM hanya cache/working state. Setiap aksi game yang diterima disimpan secara transaksional sebelum hasilnya dibroadcast.
3. **Single instance adalah target.** Redis, distributed actor, Pub/Sub, dan multi-instance bukan bagian roadmap ini. Kebutuhan scale-out hanya boleh dibuka kembali berdasarkan bukti operasional nyata dan ADR baru.
4. **REST untuk lifecycle resource; WebSocket untuk sesi realtime.** Create/join/read history dilakukan lewat HTTPS. Subscribe, presence, command game, acknowledgment, dan event dilakukan lewat WSS.
5. **Satu table actor per meja aktif.** Satu goroutine memiliki mutable state suatu meja sehingga command diproses berurutan. Setiap koneksi mempunyai read loop, write loop, dan bounded outbound queue.
6. **Semua response diproyeksikan per penerima.** North tidak pernah menerima kartu East/West/South yang masih tersembunyi. Spectator tidak diimplementasikan.
7. **Guest/basic identity only.** Identitas cukup untuk create/join, seat ownership, reconnect, dan recovery. OAuth, passkey, account linking, dan public profile tidak direncanakan.
8. **Pure engine tetap terisolasi.** Persistence, realtime, deal source, team match, dan DDS berada di application/domain boundary di luar pure game engine.
9. **Aturan online mencegah irregularity yang dapat dibuat mustahil.** Server hanya menerima call/play yang legal dan mendokumentasikan laws yang memerlukan penilaian Director atau tidak berlaku pada format ini.
10. **Team Match adalah satu-satunya format lintas meja.** Board disinkronkan pada dua meja, hasil dibandingkan dengan IMP, lalu total match disimpan. Tidak ada tournament movement atau standings umum.

### Arsitektur target ringkas

```text
Browser / PWA
  Next.js + TypeScript
        │
        ├── HTTPS: session, table lifecycle, history, health
        └── WSS: subscribe, commands, acks, ordered events
                    │
             Go modular monolith
        ┌───────────┼──────────────────────────┐
        │           │                          │
  HTTP/API      WS gateway                Table actors
        │           │                          │
        └───────────┴──────────┬───────────────┘
                               │ transactional event + snapshot
                         PostgreSQL

DDS service/library boundary
  ← completed board or explicit analysis request
  → double-dummy table, makeable contracts, par result
```

---

## 2. Asumsi produk yang dikunci untuk MVP

Tanpa batas ini, istilah “web app bridge” terlalu luas. Baseline MVP memakai asumsi berikut:

- Permainan adalah **Contract Bridge dengan duplicate scoring**, satu meja berisi North, East, South, West.
- Satu session meja memainkan satu atau beberapa board secara berurutan; MVP dianggap sukses setelah minimal satu board selesai dan tersimpan.
- Empat seat dapat diisi participant manusia atau bot sederhana yang dikelola owner; bot memilih legal call/card pertama dari engine.
- Declarer mengontrol kartu dummy setelah opening lead; pemilik seat dummy melihat permainan tetapi tidak mengirim `play_card` selama menjadi dummy.
- Deal berasal dari source eksplisit: CSPRNG aman, fixture deterministik untuk test, prepared deal, atau configurable constraint source. Setiap board menyimpan provenance dan tidak dapat dipilih ulang setelah dimulai.
- MVP mendukung auction legal, double, redouble, passed-out board, play 13 tricks, vulnerability, dan duplicate score.
- Semua WBF law yang mekanis dan relevan dengan format online ini dipetakan ke invariant, command validation, atau test. Irregularity yang UI/server dapat cegah tidak diberi alur rectification.
- Laws yang bergantung pada Director, disputed facts, unauthorized information, tempo/conduct, disclosure judgment, adjusted score, atau tournament procedure didokumentasikan sebagai `director-judgement` atau `not-applicable`, bukan dipaksakan ke engine.
- **Claim dan undo berbasis persetujuan tersedia; baseline saat ini menonaktifkannya ketika bot duduk. ENG-03 yang blocked sampai UX-G1 akan mengganti baseline itu dengan kebijakan consensus bot deterministik. Director adjustment UI, alert/convention system, tournament movement selain internal Team Match, ranking, chat, spectator, strategic/AI bot, voice/video, dan uang nyata bukan scope.** Claim hanya dapat diajukan pada batas trick dan undo hanya menargetkan aksi game terakhir milik pengaju.
- Table bersifat unlisted dan masuk melalui invite link/kode; table tidak dapat dienumerasi.
- Bahasa awal Indonesia, tetapi semua copy memakai message key agar English dapat ditambahkan tanpa membongkar UI.
- Desktop dan mobile browser modern didukung. Setelah Objective GUX selesai, semua kartu playable wajib mendukung Pointer Events drag pada desktop/touch melalui command path yang sama dengan click/tap; keyboard/click/tap tetap tersedia.

Keputusan produk aktif dicatat pada **Decision register** di section 20. Perubahan sesudahnya wajib melalui ADR/product change yang menyebut dampak terhadap protocol, data, UI, dan test.

---

## 3. Product outcome dan Objective of Done

### 3.1 Product outcome

Empat orang dengan guest/basic identity dapat memainkan duplicate bridge secara realtime dengan state durabel dan aman, lalu dua tim dapat memainkan board yang sama pada dua meja dan memperoleh hasil IMP yang tersimpan. Board selesai dapat dianalisis melalui DDS tanpa mencemari pure game engine.

### 3.2 Objective of Done untuk MVP

MVP dinyatakan **done**, bukan sekadar “fitur tampak jalan”, hanya jika seluruh kondisi berikut terbukti:

- [ ] Empat browser profile berbeda dapat create → join → seat → ready → auction → play → score → next/finish tanpa intervensi admin.
- [ ] Semua WBF law yang mekanis dan relevan dipetakan ke enforcement/test; law yang memerlukan Director atau tidak berlaku tercatat pada compliance matrix.
- [ ] Tidak ada client yang menerima hand lawan sebelum informasi itu menjadi publik menurut aturan.
- [ ] Duplicate command, double-click, pesan out-of-order, dan retry tidak menggandakan aksi.
- [ ] Reload, pindah jaringan, laptop sleep, WebSocket putus, dan backend restart dapat dipulihkan dari revision terakhir yang sudah commit.
- [ ] Satu seat tidak dapat dimainkan dua controller sekaligus; device terbaru mendapat takeover otomatis dengan fencing epoch.
- [ ] Aksi tidak sah menghasilkan error terstruktur tanpa mengubah revision/state.
- [ ] Origin allowlist, auth per message, schema validation, size limit, rate limit, redacted logs, dan graceful shutdown aktif.
- [ ] `go test -race ./...`, unit/integration test, contract test, serta E2E four-player critical path lulus di CI.
- [ ] Transisi leave/join/next-board/table-switch tidak meninggalkan seat, credential, connection, atau projected state dari room sebelumnya.
- [ ] Secure random, deterministic test, prepared, dan configurable constraint deal source menghasilkan board berprovenance jelas dengan dealer/vulnerability/identity tetap stabil.
- [ ] DDS menghasilkan double-dummy table, makeable contracts, dan par result melalui analysis boundary terpisah.
- [ ] Internal Team Match menyinkronkan board pada dua meja, mengumpulkan kedua result, menghitung IMP/total secara tepat satu kali, dan menyimpan hasil final.
- [ ] Smoke test kapasitas closed beta yang realistis lulus tanpa race, deadlock, unbounded queue, atau hidden-data leak; tidak ada target internet-scale.
- [ ] Log/metrics minimum cukup untuk mendiagnosis koneksi, reconnect, revision conflict, reject reason, DB failure, dan panic.
- [ ] Migration forward dan restart-recovery procedure diuji; satu simulasi restart di tengah board dan satu di tengah match berhasil.
- [ ] Tidak ada secret di repository atau bundle browser; dependency security scan tidak memiliki temuan critical/high yang belum ditriase.
- [ ] Runbook deploy, rollback aplikasi, dan recovery board/match tersedia; machinery SLO publik dan disaster recovery lanjutan tidak diperlukan.

### 3.3 Success metrics closed beta

Metrics bukan syarat tunggal engineering, tetapi menentukan apakah produk layak masuk fase berikutnya:

| Metric | Target awal |
|---|---:|
| Create-to-four-seated completion | ≥ 60% untuk sesi yang mengundang 4 orang |
| Four-seated-to-board-completed | ≥ 80% |
| Crash/restart causing committed action loss | 0 |
| Hidden-hand exposure incident | 0 |
| Reconnect berhasil tanpa refresh manual | ≥ 95% |
| Median waktu create sampai semua seated | diukur dulu; target ditentukan setelah 20 sesi |
| Internal error per 100 accepted commands | < 1 |
| Team match dengan board/result mismatch | 0 |
| Duplicate IMP application | 0 |

---

## 4. Scope MVP dan non-goals

### 4.1 Masuk MVP

- Landing/join flow singkat dan status koneksi.
- Guest/basic session dengan nickname.
- Create table, secure invite link, join by code, pilih/pindah/keluar seat sebelum start, dan isi vacancy ketika table aktif.
- Owner controls minimum: lock/unlock table, remove participant saat waiting/active, transfer master, start ketika tepat empat seat ready, terminate waiting table.
- Owner dapat menambah bot pada kursi kosong, mengeluarkan bot, atau mengeluarkan participant non-owner dan menggantinya dengan bot pada kursi yang sama.
- Lobby presence dan connection state.
- Contract Bridge game engine lengkap untuk satu board.
- Realtime auction, card play, trick result, board result, dan rematch/next board sederhana.
- Reconnect/resume dan multi-tab fencing.
- Persisted event history dan server snapshot; result page yang aman.
- WBF compliance matrix dan enforcement untuk seluruh law mekanis yang relevan.
- DDS analysis untuk double-dummy table, makeable contracts, dan par result melalui boundary terpisah.
- Internal Team Match dua tim/dua meja dengan synchronized boards, IMP, total, dan persisted result.
- Claim/concede sisa trick dengan persetujuan kedua lawan serta undo aksi game terakhir dengan persetujuan tiga pemain lain.
- Secure random, deterministic test, prepared, dan configurable constraint deal source dengan provenance per board.
- Responsive UI, keyboard path, reduced motion, contrast/focus dasar menuju WCAG 2.2 AA.
- Structured logging, metrics minimum, health/readiness, dan audit minimum untuk aksi owner.
- CI, migrations, dan deploy closed beta sederhana.

### 4.2 Sengaja tidak masuk MVP

- OAuth/magic link dan profil permanen.
- Payment, subscription, billing, paid plan, donation checkout, coupon, invoice, atau monetisasi dalam bentuk apa pun.
- Club/workspace/tenant dan organisasi komunitas.
- Tournament movement, pair event, Swiss/round-robin, matchpoint standings, dan Director console. Internal Team Match dua meja tetap masuk scope.
- Matchmaking/rating/leaderboard.
- Chat/DM, friend list, notification email/push.
- Spectator live, replay publik, shareable hand record, dan anti-collusion signals di luar pencegahan hidden-hand leakage.
- Strategic/AI bot, bidding hints, dan AI review. Bot first-legal-move sederhana tetap masuk scope; DDS analysis tetap masuk scope.
- Director ruling, contested-claim adjudication, irregularity correction, dan rollback administratif.
- Public community ecosystem, community roles, moderation console, OAuth/passkey, public profile, analytics funnel kompleks, payment/commercial architecture, dan multi-tenancy.
- Redis, distributed actors, Pub/Sub, multi-instance, public SLO machinery, advanced disaster recovery, dan elaborate retention infrastructure.
- Native app atau offline gameplay.

Non-goal harus benar-benar tidak muncul di UI/API sampai dirancang. Ini mengurangi branch state yang tidak teruji.

---

## 5. Persona, permission, dan journey utama

### 5.1 Role

| Role | Kemampuan MVP | Batasan |
|---|---|---|
| Visitor | melihat landing/how-to/status | belum bisa mengakses meja private tanpa invite |
| Guest participant | join, pilih seat, ready, bermain, reconnect | identitas device-scoped; tidak punya history lintas device |
| Table owner | kemampuan guest + lock, remove saat waiting/active, transfer master, start, terminate waiting table | tidak dapat mengubah hasil atau melihat hidden hand seat lain |

Authorization selalu diperiksa di server untuk setiap REST mutation dan WS command. Label owner di UI bukan authorization.

### 5.2 Happy path end-to-end

1. Visitor membuka landing; frontend memanggil `/health/ready` dengan timeout pendek.
2. Jika backend cold, UI menampilkan warming state dan retry dengan backoff.
3. Visitor memilih nickname; API membuat guest session dan device credential.
4. Owner membuat table; server menghasilkan `table_id` UUID internal dan invite code acak berentropi tinggi.
5. Owner membagikan invite URL. Tiga guest join dan memilih seat kosong.
6. Semua menekan ready. Owner menekan start; server atomically memverifikasi empat seat unik dan ready.
7. Server membuat board, dealer, vulnerability, deal rahasia, dan revision awal.
8. Semua koneksi menerima snapshot yang sudah diproyeksikan khusus penerima.
9. Command auction/play masuk berurutan, divalidasi, disimpan, di-ack, lalu event dibroadcast.
10. Setelah 13 trick, server menghitung score, menyimpan result, dan menampilkan summary.
11. Owner memilih next board/rematch atau finish. Semua participant dapat melihat history meja yang diizinkan.

### 5.3 Guest recovery

- Browser menyimpan device credential yang tidak dapat dipakai sebagai admin credential.
- Seat reservation terikat pada `guest_session_id`, bukan connection ID atau nickname.
- Reconnect meminta short-lived WebSocket ticket, lalu mengirim `resume` dengan `table_id` dan `last_seen_seq`.
- Jika credential masih valid, server mengembalikan missing events atau snapshot terbaru.
- Jika browser storage hilang, invite saja **tidak** mengambil seat yang masih terisi. Disconnect hanya mengubah presence; pelepasan seat memerlukan leave, owner removal, atau lifecycle expiry eksplisit.
- Guest credential dirotasi setelah recovery sensitif dan dapat dicabut.

---

## 6. Arsitektur aplikasi

### 6.1 Bentuk repository yang disarankan

```text
bridgeyok/
├── apps/
│   ├── web/                    # Next.js, TypeScript, UI/E2E hooks
│   └── api/                    # Go main, HTTP + WebSocket server
├── internal/                   # atau di bawah apps/api/internal
│   ├── bridge/                 # pure domain: cards, auction, play, score
│   ├── table/                  # lifecycle + actor
│   ├── match/                  # internal Team Match + IMP comparison
│   ├── deal/                   # deal sources + provenance
│   ├── analysis/               # DDS adapter boundary
│   ├── realtime/               # protocol, connection, hub, projector
│   ├── identity/               # guest/session/ticket
│   ├── persistence/            # PostgreSQL repositories/UoW
│   └── observability/
├── contracts/
│   ├── openapi.yaml            # REST contract
│   └── websocket/              # JSON Schema + protocol examples
├── db/
│   ├── migrations/
│   └── queries/
├── deploy/                     # single-instance deployment config
├── docs/
│   ├── adr/
│   ├── product/
│   └── operations/
├── .github/workflows/
├── Makefile
└── PLAN.md
```

Jika Go module tidak nyaman mengakses `internal/` di root, letakkan paket Go di `apps/api/internal` dan pertahankan boundary yang sama.

### 6.2 Technology baseline

| Area | Pilihan | Alasan |
|---|---|---|
| Web | Next.js Active LTS + TypeScript strict | routing, SSR/SEO landing, deployment dan ecosystem matang |
| Client state | TanStack Query untuk server resource; reducer/store kecil untuk projected game state | cache REST dipisah dari ordered realtime state |
| Styling/UI | React Aria Components + Tailwind CSS | accessible primitives dan satu styling/design-token approach |
| API | Go stable yang dipin, `net/http` + `go-chi/chi/v5` | router ringan; Gin tidak digunakan sesuai OD-10 |
| WebSocket | `github.com/coder/websocket` | API idiomatis, context support, ping/pong, close handshake |
| Database | PostgreSQL/Supabase, akses hanya dari Go | satu durable source of truth, transaksi, constraint, portability |
| Query/migration | `sqlc` + `pgx`, migration tool tunggal | typed query tanpa ORM state magic |
| Contract | OpenAPI untuk REST; JSON Schema/AsyncAPI-style docs untuk WS | contract test dan generated TS types |
| Observability | structured JSON logs + OpenTelemetry-compatible metrics/traces | provider dapat diganti |
| Test | Go test/property/fuzz, Vitest, Playwright | domain, component, dan multi-browser E2E |

Versi exact dipin saat bootstrap, bukan ditulis permanen di plan. Security patch harus dapat dinaikkan otomatis lewat dependency bot dan CI.

### 6.3 Boundary modul

- `bridge`: pure, deterministic, tidak mengetahui HTTP, WS, database, user, atau clock global.
- `table`: orchestrates seat, ready, board lifecycle, actor ownership, timer abstraction.
- `match`: orchestrates two-team/two-table board pairing, result comparison, dan IMP total.
- `deal`: memilih source dan menghasilkan validated deal + provenance di luar pure engine.
- `analysis`: memetakan immutable deal ke/dari DDS; tidak mengubah authoritative game state.
- `realtime`: transport dan projection; tidak menghitung aturan bridge.
- `identity`: guest/device session, recovery, dan WS ticket.
- `persistence`: repository dan transaksi; tidak mengandung keputusan UI.
- `web`: render projected state; tidak mengimplementasi legality authoritative.

Dependency mengarah ke domain. Domain tidak mengimpor infrastructure.

### 6.4 Source of truth dan write path

Untuk setiap command game:

```text
WS command
  → parse envelope + schema/size validation
  → authenticate connection + authorize seat/action
  → enqueue ke table actor
  → reject jika request_id pernah diproses (return ack sebelumnya)
  → check expected_revision
  → pure engine menghasilkan next_state + domain_events
  → PostgreSQL transaction:
       lock/check current revision
       INSERT game_event(s), unique(session_id, request_id)
       UPDATE game_snapshot SET state, revision
       optional INSERT outbox notification
  → commit
  → swap actor state
  → ack pengirim
  → project + broadcast event ke setiap penerima
```

Prinsip penting:

- Broadcast tidak boleh mendahului commit. UI tidak boleh melihat aksi yang kemudian hilang.
- `revision` monotonic per table/game. `seq` monotonic per event stream.
- Jika DB gagal, state RAM tidak diganti; pengirim mendapat retryable internal error dengan correlation ID.
- Jika broadcast gagal setelah commit, reconnect/resync membaca state yang benar dari DB.
- Actor idle dieviction dari RAM setelah TTL; saat dibutuhkan, state dihydrate dari snapshot dan event tail.
- Snapshot lengkap bersifat server-private. Projection publik tidak disimpan ulang sebagai authoritative state.
- Event log mencatat aksi domain yang sudah diterima; data rahasia seperti seed/deal tidak pernah ditulis ke application log atau analytics.

### 6.5 Boundary deployment single-instance

Roadmap menargetkan satu backend instance. PostgreSQL menyediakan durabilitas; actor, presence, dan broadcast tetap lokal. Redis, lease lintas instance, distributed actor, Pub/Sub, broker, dan multi-instance tidak dibuat maupun diabstraksikan secara spekulatif. Jika closed beta kelak membuktikan single instance tidak cukup, kebutuhan itu memerlukan profiling, target konkret, dan ADR baru di luar roadmap ini.

### 6.6 PostgreSQL connection strategy

- Gunakan pooled connection string Supabase/Supavisor sesuai mode yang kompatibel dengan transaksi aplikasi.
- Pool Go dibuat kecil dan eksplisit; jangan samakan jumlah WebSocket dengan jumlah DB connection.
- Set connection/query/transaction timeout.
- Tidak ada transaksi yang hidup selama menunggu input user atau broadcast network.
- Migration memakai koneksi/session yang sesuai dan tidak dijalankan bersamaan oleh banyak instance.
- Client browser tidak mendapat Supabase service key dan tidak melakukan query langsung ke table game.

---

## 7. Arsitektur WebSocket modern

### 7.1 Connection lifecycle

```text
DISCONNECTED
  → GET/POST session bootstrap
  → POST /v1/realtime/tickets
  → CONNECTING wss://api.../v1/ws?ticket=<single-use>
  → AUTHENTICATED
  → SUBSCRIBE table + last_seen_seq
  → SYNCING (snapshot atau missing events)
  → LIVE
  → DEGRADED/OFFLINE
  → exponential backoff + full jitter
  → ticket baru → resume
```

Browser WebSocket API tidak dapat mengirim arbitrary Authorization header. Karena frontend dan API free-tier bisa berada pada domain provider yang berbeda, baseline aman adalah:

1. Auth HTTPS memperoleh/menjaga guest credential.
2. Client meminta **single-use WS ticket** dengan TTL sangat pendek (misalnya 30–60 detik).
3. Ticket boleh berada di query handshake hanya jika single-use, hash-nya disimpan server-side, dan URL/query disensor dari log.
4. Provider subdomain tetap dipakai agar biaya Rp0; auth tidak bergantung pada shared-domain cookie. Origin validation tetap wajib.

Jangan menaruh long-lived access token di URL, local log, error report, atau analytics.

### 7.2 Envelope protocol v1

Semua application message berupa JSON kecil dengan envelope konsisten:

```json
{
  "v": 1,
  "kind": "command",
  "name": "play_card",
  "request_id": "01J...",
  "table_id": "01J...",
  "expected_revision": 37,
  "sent_at": "2026-08-29T12:00:00Z",
  "payload": { "card": "SA" }
}
```

Server membalas salah satu/lebih frame berikut:

```json
{
  "v": 1,
  "kind": "ack",
  "request_id": "01J...",
  "status": "accepted",
  "revision": 38,
  "seq": 81
}
```

```json
{
  "v": 1,
  "kind": "event",
  "name": "card_played",
  "table_id": "01J...",
  "revision": 38,
  "seq": 81,
  "server_time": "2026-08-29T12:00:00Z",
  "payload": { "seat": "N", "card": "SA" }
}
```

```json
{
  "v": 1,
  "kind": "error",
  "request_id": "01J...",
  "code": "MUST_FOLLOW_SUIT",
  "retryable": false,
  "revision": 37,
  "message_key": "game.error.must_follow_suit"
}
```

Aturan protocol:

- `request_id` dibuat client, unik per session, dan dipakai untuk idempotency.
- `expected_revision` mencegah stale UI mengeksekusi intent pada state baru.
- `seq` menentukan gap/replay, bukan timestamp.
- `message_key`, bukan arbitrary backend text, dipakai untuk copy user-facing.
- Unknown field boleh diabaikan dalam minor-compatible change; unknown `v`/kind/name yang wajib dipahami ditolak.
- Breaking change memakai protocol version baru dengan masa overlap terukur.
- Maksimal payload awal 8 KiB; tidak ada binary frame di MVP; per-message compression default off sampai threat/performance test membenarkannya.
- Server tidak mempercayai `sent_at`; itu hanya telemetry, tidak menentukan legality atau timeout.

### 7.3 Command dan event minimum

**Client commands**

- `subscribe_table`, `resume`
- `take_seat`, `leave_seat`, `set_ready`
- `lock_table`, `remove_participant`, `start_game`, `finish_table`
- `add_bot`, `remove_bot`, `replace_with_bot`
- `make_call` (`PASS`, bid, `DOUBLE`, `REDOUBLE`)
- `play_card`
- `request_next_board`
- `client_heartbeat`

**Server events**

- `snapshot`, `resync_required`
- `participant_joined`, `participant_left`, `presence_changed`
- `seat_changed`, `ready_changed`, `table_locked`
- `board_started`, `call_made`, `auction_completed`, `board_passed_out`
- `card_played`, `dummy_revealed`, `trick_completed`
- `board_scored`, `table_finished`
- `controller_replaced`, `server_draining`
- `ack`, `error`, `heartbeat_ack`

Presence event tidak mengubah domain revision. Game/lobby mutations mengubah revision. Pemisahan ini mencegah heartbeat mengotori durable event log.

### 7.4 Connection implementation

Per connection:

- Satu goroutine membaca dan memvalidasi frame.
- Satu goroutine menulis dari bounded queue; tidak ada banyak writer liar.
- Queue mempunyai batas message dan byte. Slow consumer tidak boleh menghabiskan RAM.
- Jika queue penuh, drop hanya event presence yang dapat direkonstruksi. Untuk domain event, tutup koneksi dengan reason `SLOW_CONSUMER`; client reconnect dan resync.
- Set read limit sebelum membaca message.
- Server protocol ping berkala mendeteksi koneksi mati; browser menjawab pong otomatis.
- Karena browser JavaScript tidak dapat mengirim control-frame ping, client juga mengirim application `client_heartbeat` dengan jitter hanya selama screen realtime aktif. Heartbeat tidak menyentuh DB/Redis.
- Read deadline diperpanjang pada pong/application heartbeat. Interval dan deadline menjadi config, bukan magic number.
- Close memakai close handshake dan code/reason yang aman, lalu cancel context dan unsubscribe.
- SIGTERM: stop menerima koneksi/command baru, kirim `server_draining`, tunggu transaksi aktif, persist state, close socket, exit di bawah batas platform.

### 7.5 Ordering, duplicate, dan resync

- Actor memproses satu command per table pada satu waktu.
- Unique constraint `(session_id, request_id)` menyimpan hasil command untuk retry.
- Event dengan `seq <= last_applied_seq` diabaikan client.
- Event dengan `seq > last_applied_seq + 1` membuat client berhenti mengirim mutation dan meminta resync.
- Revision conflict mengembalikan `STATE_CHANGED` plus revision terbaru; client apply snapshot/event terlebih dahulu, lalu user mengulang intent jika masih relevan.
- Command tidak otomatis di-retry kecuali protocol memastikan idempotency. UI boleh retry request ID yang sama.
- Snapshot menyertakan `revision`, `last_seq`, `server_time`, `connection_epoch`, projected participants, public game state, dan private hand penerima.

### 7.6 Multi-tab dan fencing

- Satu identity boleh membuka beberapa connection.
- Hanya satu `controller_epoch` aktif per seat.
- Tab/device baru untuk guest yang sudah duduk otomatis mengirim satu takeover setelah projection fresh.
- Setelah takeover, command dari epoch lama ditolak `STALE_CONTROLLER`, meskipun socket lama belum mati.
- Pergantian controller tidak memerlukan tombol atau toast konfirmasi dan tetap dicatat tanpa membocorkan token.

---

## 8. Game domain dan state machine

### 8.1 Aggregate state

```text
TABLE_WAITING
  → TABLE_READY
  → BOARD_DEALING
  → AUCTION
      ├── PASSED_OUT → BOARD_SCORED
      └── CONTRACT_SET → OPENING_LEAD → PLAY
                                      → TRICK_1..13
                                      → BOARD_SCORED
  → BETWEEN_BOARDS
      ├── BOARD_DEALING
      └── TABLE_FINISHED

Any nonterminal state
  → TABLE_ABANDONED (hanya oleh expiry/admin policy yang terdokumentasi)
```

Transition hanya terjadi lewat domain method. Database status atau frontend tidak boleh diubah ad hoc.

### 8.2 Invariant kartu/deal

- Deck tepat 52 kartu unik, 13 per seat.
- Production random source memakai `crypto/rand`/CSPRNG server, tidak `math/rand` dengan seed waktu. Deterministic/test, prepared, dan constraint source masuk melalui application-level deal boundary.
- Setiap board menyimpan source type/version dan provenance yang cukup untuk audit/reproduction sesuai source, tanpa mengekspos seed/hand.
- Dealer dan vulnerability mengikuti board number cycle yang ditetapkan.
- Deal tersimpan durabel sebelum `board_started` dibroadcast.
- Setiap card selalu tepat di satu lokasi: hand, current trick, atau completed trick.
- Client hanya menerima hand sendiri; dummy baru diproyeksikan kepada semua setelah opening lead diterima.
- Setelah board selesai, kebijakan reveal seluruh hand harus eksplisit. Default MVP: hanya participant meja dapat melihat full deal di result.
- Raw deal/seed tidak masuk application log, trace, error payload, analytics, atau client cache pihak lain.

### 8.3 Invariant auction

- Turn dimulai dari dealer dan berputar N → E → S → W.
- Call adalah Pass, bid level 1–7 + strain, Double, atau Redouble.
- Bid baru harus lebih tinggi berdasarkan level dan urutan strain yang dikunci engine.
- Double hanya legal terhadap contract terakhir lawan dan saat belum doubled/redoubled.
- Redouble hanya legal oleh pihak yang didouble dan belum redoubled.
- Empat pass awal menghasilkan passed-out board.
- Setelah sebuah bid, tiga pass berurutan menutup auction.
- Declarer adalah pemain pertama dari partnership pemenang contract yang pernah menyebut strain final.
- Opening leader adalah seat di kiri declarer.
- Aksi auction setelah selesai ditolak tanpa perubahan state.

Irregularities resmi yang memerlukan director (call out of turn, insufficient bid yang diterima lawan, penalty) **tidak disimulasikan**: UI hanya menawarkan aksi legal dan server menolak aksi ilegal. Ini adalah aturan produk online MVP, bukan implementasi penuh turnamen ber-director.

### 8.4 Invariant play

- Opening leader memainkan kartu pertama.
- Setelah opening lead, dummy terbuka dan declarer mengontrol kartu dummy.
- Hanya controller seat yang berhak—atau declarer saat giliran dummy—dapat mengirim `play_card`.
- Pemain wajib follow suit jika masih memiliki suit led.
- Empat kartu tepat per trick; pemenang trick memimpin trick selanjutnya.
- Trump mengalahkan non-trump; di antara suit yang relevan, rank tertinggi menang.
- Satu kartu tidak pernah dapat dimainkan dua kali.
- Setelah 13 trick, jumlah trick NS + EW = 13 dan tidak ada hand tersisa.
- Play setelah terminal state ditolak.
- Claim/concede hanya selesai bila kedua lawan menerima; penolakan melanjutkan play tanpa adjudication. Undo hanya memulihkan snapshot sebelum aksi game terakhir setelah tiga pemain lain menerima. Revoke correction dan timeout auto-play tidak tersedia.

### 8.5 Scoring

- Gunakan duplicate contract scoring dan test table-driven untuk made/down, doubled/redoubled, vulnerable/non-vulnerable, partscore/game/slam bonus, overtrick, undertrick, dan insult bonus.
- `score_ns` menjadi representasi canonical signed integer; score EW adalah kebalikannya.
- Result menyimpan contract, declarer, double state, tricks won, vulnerability, score, dan engine ruleset version.
- Ruleset version tidak berubah di tengah board. Reprocessing history harus memakai versi semula.
- Golden test cases diverifikasi terhadap referensi WBF dan set kasus independen yang direview pemain bridge berpengalaman.
- Team Match comparison memakai WBF Law 78B IMP scale di module `match`, bukan di pure single-board score engine. Inputnya dua `score_ns` yang sudah final dan orientation team yang eksplisit.

### 8.6 Pure engine API

Contoh boundary:

```go
type Command struct {
    ActorSeat Seat
    Name      CommandName
    Payload   any
}

type Decision struct {
    NextState State
    Events    []Event
}

func Decide(state State, command Command) (Decision, *DomainError)
```

- Fungsi deterministic: state + command yang sama menghasilkan decision yang sama.
- Clock, ID, dan RNG diinjeksikan dari luar.
- State divalidasi dengan `ValidateInvariants` dalam test/debug path.
- Event reducer dapat membangun state dari fixture/event history untuk audit dan replay test.

### 8.7 WBF compliance dan DDS boundary

- Compliance matrix Phase 4 adalah sumber kebenaran coverage laws dan mempunyai tiga status saja: `mechanically-enforced`, `director-judgement`, dan `not-applicable`.
- `mechanically-enforced` berarti server mencegah tindakan ilegal sebelum menjadi event authoritative; tidak dibuat alur irregularity/rectification untuk aksi yang dapat dibuat mustahil.
- `director-judgement` mencakup law yang bergantung pada penilaian fakta, damage/equity, unauthorized information/tempo/conduct, disclosure, adjusted score, atau discretionary rectification.
- `not-applicable` mencakup physical/tournament procedure yang tidak ada pada produk terfokus ini. Klasifikasi harus menyebut alasan, bukan hanya nomor law.
- DDS menerima immutable deal melalui `analysis` boundary dan hanya mengembalikan double-dummy table, makeable contracts, serta par result. DDS tidak menentukan legal command, score authoritative, revision, atau state transition engine.

---

## 9. Data model awal

Semua primary ID memakai UUID/ULID non-sequential yang aman diekspos; invite code bukan primary key.

| Table | Isi penting | Constraint/retention |
|---|---|---|
| `guest_sessions` | id, credential hash, nickname, status, created/last_seen/expires | credential plaintext tidak disimpan; TTL |
| `tables` | id, owner session, invite code hash, state, locked, revision, timestamps | invite code unique; state check |
| `table_participants` | table, session, role, joined/left | satu participant aktif per session/table |
| `table_seats` | table, seat N/E/S/W, participant, ready, controller epoch | unique table+seat dan table+participant |
| `boards` | identity, table/match reference, number, dealer, vulnerability, ruleset, deal source/provenance, status, result | identity stabil; unique dalam scope |
| `game_snapshots` | board/table, revision, last_seq, private state JSONB, updated | server-only; one current row per aggregate |
| `game_events` | aggregate, seq, revision, request/session, event type/payload, occurred | unique aggregate+seq; unique session+request |
| `processed_commands` | idempotency result/expiry bila tidak digabung event | bounded retention |
| `realtime_tickets` | ticket hash, session, expiry, used_at | single-use; short TTL |
| `team_matches` | owner, two teams, state, board set, IMP total, final result | one persisted result per match |
| `team_match_tables` | match, room/table, team/room assignment | exactly required open/closed-room mapping |
| `team_match_board_results` | match board, room/table result, score, comparison/IMP status | unique result per room; comparison idempotent |

Database rules:

- Migration append-only dan reviewable; production migration tidak mengandalkan dashboard click.
- Foreign key dan check constraint menjaga invariant struktural.
- Index minimal pada invite lookup, participant session, active status, event aggregate+seq, expiry cleanup.
- Invite code dan credential disimpan sebagai keyed/hash verifier bila tidak perlu dikembalikan lagi.
- JSONB dipakai untuk versioned private snapshot, bukan sebagai alasan menghilangkan relational constraints seluruh domain.
- `updated_at` bukan ordering mechanism realtime.
- Retention cukup memakai cleanup sederhana untuk expired ticket/session dan data closed beta. Kebijakan retention lanjutan tidak dibuat sebelum kebutuhan nyata.

### Projection model

Satu fungsi wajib menghasilkan view dari authoritative state:

```text
Project(state, viewerIdentity)
  → own hand (jika seated)
  → dummy hand (jika already revealed)
  → played cards / auction / score publik
  → no opponent hidden cards
  → no credential, IP, internal note, RNG/deal secret
```

Semua snapshot, reconnect, REST history, admin preview, dan WS event memakai projector yang dites. Tidak boleh ada handler yang menyusun response game secara manual.

---

## 10. REST API minimum

```text
GET    /health/live
GET    /health/ready
POST   /v1/guest-sessions
POST   /v1/guest-sessions/refresh
DELETE /v1/guest-sessions/current

POST   /v1/tables
GET    /v1/tables/{invite_code}/preview
POST   /v1/tables/{invite_code}/join
GET    /v1/tables/{table_id}
POST   /v1/tables/{table_id}/leave
GET    /v1/tables/{table_id}/boards/{board_id}/result

POST   /v1/realtime/tickets
GET    /v1/ws                             # HTTP upgrade only

POST   /v1/team-matches
GET    /v1/team-matches/{match_id}
POST   /v1/team-matches/{match_id}/start
GET    /v1/boards/{board_id}/analysis
```

Guidelines:

- Mutation response membawa `request_id/correlation_id` dan revision bila relevan.
- Error mempunyai stable `code`, safe `message_key`, `retryable`, field violations opsional.
- `404`/join error tidak membedakan “kode pernah ada”, “locked”, atau “banned” sebelum caller terotorisasi jika perbedaan itu membantu enumeration.
- CORS exact allowlist; credentials hanya pada origin resmi.
- Health `live` tidak menyentuh dependency; `ready` memeriksa kesiapan dependency dengan timeout.
- OpenAPI adalah contract dan lulus breaking-change check di CI.

---

## 11. Edge-case catalog closed beta

Catalog ini menjadi sumber test case dan harus terus bertambah ketika bug ditemukan. Kolom “perilaku wajib” adalah acceptance behavior, bukan saran UI semata.

### 11.1 Entry, identity, dan invite

| Kondisi | Perilaku wajib |
|---|---|
| Nickname kosong/terlalu panjang/control char | normalisasi Unicode, trim, batas 2–24 grapheme, reject control/bidi abuse; render sebagai text, bukan HTML |
| Nickname sama | boleh dengan discriminator singkat; identity tetap berdasarkan ID |
| Invite salah/expired/diubah case | response generik, tidak leak existence; format code case-insensitive bila ditetapkan |
| Brute-force invite | high-entropy code dan rate limit IP/session |
| Link dibuka messaging preview bot | GET preview tidak join, claim seat, atau memakai one-time token |
| Browser menolak storage | beri warning “recovery terbatas”; session tetap dapat berjalan in-memory |
| Guest membuka link dari device lain | dianggap identity baru; tidak otomatis mengambil seat lama |
| Credential dicuri/replay | hash-at-rest, rotation/revoke, short session policy, controller fencing, audit signal |
| User logout/clear data saat seated | socket ditutup; seat tetap durabel sampai leave, owner removal, atau lifecycle expiry eksplisit |
| Session expired saat socket hidup | server revalidate/expiry timer; minta refresh atau close dengan code terstruktur |

### 11.2 Lobby, seat, dan start race

| Kondisi | Perilaku wajib |
|---|---|
| Dua user mengambil seat yang sama | satu transaksi menang; yang kalah mendapat `SEAT_TAKEN` + state terbaru |
| Satu user mengambil dua seat | constraint menolak; satu participant hanya satu seat |
| User pindah seat saat ready | ready reset; event tunggal yang konsisten |
| User disconnect sebelum start | presence menjadi `offline`; participant dan seat tetap durabel |
| Owner disconnect | presence menjadi `offline`; ownership dan seat tidak berubah tanpa lifecycle command eksplisit |
| Owner kick dirinya sendiri | ownership wajib ditransfer atomik ke participant aktif lain; ditolak bila tidak ada pengganti |
| Kick saat board berjalan | tersedia bagi owner; seat menjadi vacancy tanpa mengubah call/card game |
| Start diklik dua kali | idempotent; hanya satu board/deal dibuat |
| Start bersamaan dengan leave/unready | revision/transaction menentukan satu urutan; start hanya commit jika empat seat masih ready |
| Join ke table locked/full/started | locked waiting/full tetap safe error; started table menerima identity baru hanya bila ada vacancy |
| Table kosong | expire otomatis; tidak menyisakan actor/goroutine |
| Kode invite dibagikan publik | owner dapat rotate/lock sebelum start; existing participant tidak terlempar tanpa aksi jelas |

### 11.3 Auction dan play

| Kondisi | Perilaku wajib |
|---|---|
| Command bukan giliran user | reject `NOT_YOUR_TURN`, no revision change |
| Bid insufficient, double/redouble ilegal | reject domain code spesifik, no mutation |
| Empat pass pertama | board passed out dan score/result terminal yang benar |
| Tiga pass setelah contract | declarer/opening leader dihitung konsisten |
| User memainkan kartu yang tidak dimiliki | reject dan security counter naik |
| User gagal follow suit padahal bisa | reject `MUST_FOLLOW_SUIT` tanpa membocorkan hand selain fakta yang sudah implisit bagi pemilik hand |
| Declarer mencoba main dari hand saat giliran dummy | reject; UI menunjukkan hand aktif |
| Dummy mencoba memainkan kartunya | reject `DECLARER_CONTROLS_DUMMY` |
| Double click/tap atau retry jaringan | request ID sama mengembalikan hasil sama; kartu hanya sekali |
| Dua command dari stale UI | command pertama commit; berikutnya `STATE_CHANGED` dan resync |
| Aksi datang sesudah board selesai | reject terminal-state; score tidak berubah |
| Engine panic/invalid invariant | table dipause fail-closed, correlation ID tercatat, tidak broadcast state parsial |
| Deal creation gagal disimpan | board tidak diumumkan; retry tidak menghasilkan dua board |
| Score boundary (7NTXX, vulnerable down banyak) | integer safe dan golden test; tidak overflow/bonus ganda |

### 11.4 Network, reconnect, multi-tab

| Kondisi | Perilaku wajib |
|---|---|
| Wi-Fi berpindah/mobile background | offline banner; exponential backoff + jitter; resume dari `last_seen_seq` |
| Socket putus setelah DB commit sebelum ack | retry request ID sama mengembalikan accepted result, bukan aksi kedua |
| Socket putus sebelum commit | result tidak pasti di client; retry ID yang sama menentukan outcome dari idempotency store |
| Event hilang/gap | client freeze mutation, resync; tidak menebak state |
| Event duplicate/out-of-order | dedupe seq; gap detection; reducer tidak korup |
| Backend restart di tengah board | hydrate snapshot/event, reconnect, semua hand dan turn sama dengan committed revision |
| Deploy/SIGTERM | server draining event, stop command, commit selesai, close/reconnect |
| DB sementara unavailable | reject/retryable; RAM tidak maju mendahului DB |
| Client sangat lambat | bounded queue; close `SLOW_CONSUMER`; resume via snapshot |
| Tab lama dan tab baru aktif | tab terbaru otomatis takeover; epoch lama read-only/rejected |
| Browser offline mengantre click | UI hanya satu pending domain command; stale command tidak dikirim massal saat online |
| System clock user salah | tidak memengaruhi ordering/rules; server time authoritative |

### 11.5 Hidden information dan privacy

| Kondisi | Perilaku wajib |
|---|---|
| User mengganti seat lewat request buatan | server authorize; tidak mengubah projector hand aktif sembarangan |
| Non-participant memanggil endpoint table | authorization menolak; tidak ada hand/data game live |
| Cache/CDN menyimpan response private | `Cache-Control: private, no-store` pada table/game/session data |
| Error/trace merekam snapshot | serializer/log filter melarang private state dan token |
| Internal product metrics merekam card/hand/token/invite | event schema allowlist; tidak mengirim atau menyimpan payload realtime mentah |
| Source map/devtools memperlihatkan key | hanya public config di bundle; secret server-only |
| Result diminta orang tanpa izin | authorization participant; invite code saja tidak otomatis memberi history penuh |
| User mencoba sequential IDs | non-enumerable IDs dan authorization tetap wajib |

### 11.6 Abuse dan resource exhaustion

| Kondisi | Perilaku wajib |
|---|---|
| Ribuan koneksi dari satu sumber | per-IP/session connection cap, handshake rate limit, idle timeout |
| Message flood | token bucket per connection/session/IP; close repeated offender |
| Frame besar/decompression bomb | read limit; compression off awal; close policy violation |
| JSON malformed/unknown command | structured reject, strike counter, tidak panic |
| Presence connect/disconnect flapping | debounce/coalesce presence; tidak persist setiap heartbeat |
| Create-table spam | rate limit per session/IP dan expiry cleanup sederhana |
| XSS/SQL injection text payload | strict schema/length, parameterized SQL, contextual output encoding, CSP |
| Cross-site WS hijacking | exact Origin allowlist, SameSite policy bila cookie, single-use ticket, per-message auth |
| CSRF REST mutation | SameSite + CSRF strategy untuk cookie; bearer endpoint tidak menerima ambient auth sembarangan |

### 11.7 Lifecycle dan operasional

| Kondisi | Perilaku wajib |
|---|---|
| Table idle sebelum start | warning lalu expire; participant mendapat terminal reason |
| Board idle karena player hilang | state dipertahankan sampai policy TTL; tidak auto-play pada MVP |
| TTL tercapai saat user baru reconnect | deterministic `TABLE_EXPIRED/ABANDONED`, bukan 500 |
| Migration incompatible dengan active socket | backward-compatible expand/migrate/contract; protocol overlap |
| DB read-only/unavailable | mutation fail-closed; state RAM tidak maju; existing read path memberi status jelas |
| Feature flag berubah tengah board | ruleset/critical flags dipin saat board dibuat |
| Restart di tengah Team Match | hydrate kedua table dan match comparison tanpa menggandakan result/IMP |

---

## 12. Security, privacy, dan resource baseline

### 12.1 Controls wajib

- WSS/HTTPS only di luar local development; HSTS diaktifkan hanya setelah behavior host/subdomain provider diverifikasi aman.
- Exact Origin allowlist pada WebSocket handshake; tidak memakai wildcard/substring.
- Single-use WS ticket, short TTL, hashed at rest, atomic consume.
- AuthN saat handshake dan AuthZ pada **setiap command**, termasuk table ID dan seat.
- Request/message JSON schema, enum, length, numeric bounds, dan maximum nesting.
- Initial rate limit guideline: create table 5/jam/session dan 10/jam/IP; join 30/10 menit/IP; WS mutation 10/detik burst 20/session; maksimum 3 socket/session dan batas wajar/IP. Angka dituning dari telemetry.
- Payload maksimum 8 KiB; idle/handshake/write timeout; bounded outbound queue.
- Parameterized SQL; least-privilege DB role; production secret dipisah per environment dan dirotasi.
- CSP, frame-ancestors, nosniff, referrer policy, secure cookie flags jika cookie digunakan.
- No private caching untuk session/table/game; public asset boleh immutable.
- Logs memakai allowlist field dan correlation ID; token, invite secret, raw IP, private hand, dan message payload penuh dilarang.
- Dependency update otomatis, SAST/secret scan, Go vuln scan, npm audit/OSV-equivalent, image/container scan bila Docker.

### 12.2 Threat model minimum

Threat modeling pada Phase 0 harus mencakup:

- seat hijacking dan session replay;
- cross-site WebSocket hijacking;
- hidden-hand exfiltration melalui projection, logs, cache, error, replay, atau admin UI;
- collusion/out-of-band communication (tidak dapat dihilangkan, hanya dibatasi/dinyatakan untuk casual play);
- invite enumeration dan scraping;
- command injection/tampering;
- connection/message/room exhaustion;
- privilege escalation owner;
- dependency/supply-chain compromise;
- malicious table state yang memicu panic saat hydrate;
- persistence/telemetry sebagai secondary data leak.

### 12.3 Privacy posture

- Data minimization: nickname, technical session metadata, dan game history saja untuk guest.
- IP tidak disimpan raw lebih lama dari kebutuhan abuse/security; bila agregasi cukup, simpan keyed hash dengan rotation.
- Closed-beta notice menjelaskan browser storage, game history, nickname yang terlihat participant, dan contact penghapusan data.
- Delete guest mempertimbangkan referential integrity: anonymize participant identity pada immutable game/match record jika penghapusan penuh merusak hasil.
- Third-party analytics dan session replay tidak digunakan pada seluruh produk.

---

## 13. Frontend dan UX state

### 13.1 Route minimum

```text
/                    landing + create/join CTA
/how-to              aturan produk yang didukung
/join/[inviteCode]    safe preview + nickname/join
/table/[tableId]      lobby/game/reconnect/result shell
/match/[matchId]      setup/progress/final Team Match
/board/[boardId]      participant-authorized result + DDS analysis
```

### 13.2 UI state yang harus terlihat

- bootstrapping session;
- joining, lobby empty/partial/full/locked;
- seated/unseated, ready/not ready, owner/non-owner;
- syncing/live/unstable/offline/reconnecting/resyncing;
- my turn/opponent turn/dummy turn;
- pending command dengan interaction lock sempit, bukan freeze seluruh page;
- domain error, auth expired, table expired/abandoned;
- board passed out/scored/finished;
- unsupported/outdated client protocol dengan hard refresh path.

### 13.3 Accessibility dan responsive baseline

- Target WCAG 2.2 AA untuk critical path.
- Semua kartu dapat dipilih keyboard; suit/rank tidak hanya dibedakan warna/simbol visual.
- Focus order mengikuti urutan permainan; focus visible tidak tertutup toast/modal.
- Live region mengumumkan bid/card/trick secara ringkas dan dapat dipause.
- Minimum target touch memadai; landscape dan portrait diuji.
- Reduced motion dihormati; animasi kartu tidak menunda state authoritative.
- Teks/ikon menjelaskan vulnerability, dealer, declarer, turn, connection state.
- Screen reader tidak membacakan hidden card yang tidak dimiliki user.
- Saat reconnect, focus tidak dilempar tanpa alasan dan pending action dijelaskan.

### 13.4 Client state discipline

- REST query cache bukan source of truth game live.
- WS reducer hanya apply event berurutan dari server.
- Optimistic UI terbatas pada visual pending; card/bid tidak dianggap final sebelum ack/event.
- Private state dibersihkan saat logout, leave, seat change, controller replacement, dan table switch.
- Service worker/PWA tidak cache API/private snapshot secara default.

---

## 14. Testing dan quality gates

### 14.0 Verification policy

Testing wajib proporsional terhadap risiko dan luas perubahan.

- Inspeksi hanya file yang langsung relevan dengan tugas dan dependency terdekatnya.
- Jangan memindai seluruh repository kecuali discovery arsitektur memang diperlukan.
- Jangan membuka ulang file yang relevan isinya sudah diketahui, kecuali ada perubahan yang memengaruhi keputusan.
- Jangan mengulang command gagal yang sama jika code/config yang relevan belum berubah.
- Jangan menjalankan full project gate untuk edit rutin.
- Hentikan verifikasi setelah acceptance criteria dan test paling murah yang tepat sudah lulus.

Hierarchy verifikasi:

1. Level 1: unit/package test terkecil yang relevan.
2. Level 2: seluruh test untuk domain terdampak bila perilaku melintasi beberapa package dalam domain itu.
3. Level 3: DB, WebSocket, persistence, restart, atau multi-client integration test hanya bila boundary itu disentuh.
4. Level 4: repository-wide test, full race suite, stress test, long fuzz, security scan, migration validation, dan check mahal lain hanya saat phase completion, sebelum release, setelah refactor lintas domain besar, saat menyelidiki failure yang relevan, atau bila diminta eksplisit.

Aturan tambahan:

- Default randomized bridge engine suite dibatasi maksimal 250 game.
- 10.000 game hanya untuk milestone engine atau stress verification yang eksplisit.
- Race detector hanya untuk package concurrency yang terdampak, bukan repository-wide setelah perubahan non-concurrent.
- Fuzz panjang tidak dijalankan pada implementasi normal; gunakan corpus/regression yang sudah ada kecuali milestone menuntut lebih.
- Security/dependency scan, migration validation, dan external integration test hanya dijalankan bila perubahan memang menyentuh area itu atau bila gate phase/release memerlukannya.
- Untuk bug yang ditemukan, prioritaskan satu regression test deterministik dibanding menaikkan budget randomized/stress berulang kali.

### 14.1 Test pyramid

**Pure engine tests**

- exhaustive representation test 52 kartu;
- table-driven legal/illegal auction;
- property: legal play selalu mengurangi tepat satu hand dan menambah tepat satu played card;
- property: semua completed deal mempunyai 13 trick dan 52 kartu unik;
- property: score EW = `-score_ns`;
- golden scoring matrix;
- deterministic replay dari event fixtures;
- Go fuzz untuk decoder command, reducer, invariant validator, dan hydrate snapshot versi lama.

**Application/integration tests**

- transaction rollback, revision conflict, idempotency unique constraint;
- two-client seat race;
- DB commit-before-broadcast;
- projection matrix N/E/S/W/owner/unseated dan isolation antar Team Match room;
- session/ticket expiry dan atomic single use;
- origin/CORS/auth/rate-limit/oversized message;
- actor creation/idle eviction/shutdown tanpa goroutine leak;
- reconnect gap, duplicate, out-of-order, slow consumer.
- deal-source provenance dan atomic board creation;
- paired Team Match result, idempotent IMP comparison, dan restart hydrate.

**DDS component tests**

- pinned representative deals untuk double-dummy table, makeable contracts, dan par result;
- adapter/API mapping dan explicit solver failure;
- dependency guard: pure engine tidak mengimpor DDS/analysis.

**Contract tests**

- semua REST response sesuai OpenAPI;
- semua WS example sesuai JSON Schema;
- generated Go/TS types sinkron;
- backward compatibility untuk protocol version yang masih disupport.

**E2E Playwright**

- empat isolated browser context menyelesaikan board fixture;
- passed-out board;
- double/redouble + vulnerable score;
- reload salah satu pemain pada auction dan play;
- network offline setelah commit sebelum ack;
- owner/start race;
- multi-tab automatic takeover;
- mobile viewport + keyboard-only critical path;
- privacy assertion: response/socket North tidak mengandung card East/West/South yang tersembunyi.
- satu Team Match happy path dengan eight isolated clients sebagai release smoke.

**Focused resilience/smoke**

- restart API setiap fase game;
- DB latency/failure injection;
- SIGTERM/drain pada koneksi aktif;
- jumlah connection/table closed beta yang direncanakan ditambah margin kecil;
- slow readers dan reconnect storm dengan jitter;
- race detector pada scenario concurrent kritis.

### 14.2 CI gate per pull request

```text
format/lint
→ generated-contract-diff check
→ unit + property subset
→ Go race test package kritis
→ integration dengan ephemeral PostgreSQL
→ web unit/component
→ build web + API
→ secret/vulnerability scan
→ E2E critical path
```

Milestone/release dapat menjalankan fuzz lebih lama, full scoring/IMP matrix, focused smoke, dan dependency scan mendalam.

### 14.3 Definition of Done setiap task

Sebuah task selesai bila:

- acceptance criteria dan failure behavior terpenuhi;
- authorization, privacy projection, observability, accessibility relevan ikut dikerjakan;
- unit/integration/contract/E2E test proporsional ditambah;
- migration backward-compatible bila ada data change;
- docs/ADR/runbook/contract diperbarui bila boundary berubah;
- lint/test/security gate lulus;
- tidak menambah unbounded queue/goroutine/cache;
- reviewer dapat menjalankan/verifikasi melalui command terdokumentasi.

---

## 15. Observability dan operasi minimum

### 15.1 Structured log fields

`timestamp`, `level`, `service`, `version`, `environment`, `request_id`, `connection_id` (ephemeral), `session_id_hash`, `table_id`, `board_id`, `command_name`, `result_code`, `revision`, `seq`, `latency_ms`.

Jangan log token, invite code penuh, raw cookie, raw hand/deal, arbitrary nickname tanpa sanitasi, atau WS payload penuh.

### 15.2 Metrics minimum

- HTTP request count/error/latency by route template.
- WS connect/reject/close by code; active connections; reconnect count.
- Active table actors, seated/ready/live tables.
- Command accepted/rejected/error and latency by command.
- Revision conflict, duplicate request, resync, seq gap.
- Outbound queue depth, slow-consumer close, dropped presence.
- DB pool usage/wait, query/transaction latency/error.
- Hydrate/recovery duration; shutdown drain duration.
- Match board paired/unpaired, comparison applied, dan match finalized.
- DDS request duration/error dan deal-source failure tanpa mencatat deal.

### 15.3 Diagnostic acceptance

- Committed game action dan finalized match result tidak hilang setelah restart.
- Log/metrics dapat membedakan transport failure, rejected domain command, revision conflict, DB failure, projector failure, DDS failure, dan panic.
- Alert minimum hanya untuk service unavailable, DB failure, hidden-data assertion, dan panic.
- Tidak ada public SLO, error-budget machinery, analytics funnel kompleks, atau capacity target internet-scale.

### 15.4 Runbook minimum

- deploy dan smoke check;
- rollback app tanpa rollback schema destruktif;
- migration atau DB failure;
- API down;
- compromised secret/session revoke;
- hidden-data incident;
- graceful drain dan recovery active table/match.

---

## 16. Environment dan deployment closed beta

### 16.1 Environment

| Environment | Tujuan | Data |
|---|---|---|
| Local | web/API/WSS/TLS dan release drill via Compose; memakai Supabase owner | data Supabase, tidak di-reset oleh gate |
| CI | integration/E2E isolated | ephemeral |
| Staging | release candidate lokal pada mesin owner | Supabase owner; migration hanya `up` |
| Closed beta | deployment single-instance yang dipilih saat release | data participant terbatas |

Secret, database, invite namespace, cookie, CORS origin, dan telemetry harus terpisah per environment.

### 16.2 Deployment awal gratis

```text
Web:       Docker Compose lokal
API/WSS:   Docker Compose lokal; probe WSS terisolasi dari API produk
Database:  Supabase Free PostgreSQL
Redis:     none
TLS:       Caddy local CA, verifikasi TLS tetap aktif
Domain:    *.bridgeyok.localhost pada port 8443
```

Konsekuensi yang diterima:

- Supabase free mempunyai limit/pause dan tidak menyediakan backup production-grade.
- Mesin owner harus hidup dan Docker harus berjalan; stack belum dapat diakses user publik.
- CA development dapat memerlukan trust manual di browser, sedangkan automated gate memakai CA eksplisit tanpa `-k`.
- Tidak ada availability/SLA publik, multi-region, advanced disaster recovery, atau elaborate retention infrastructure.
- Network exposure closed beta diputuskan saat Phase 5 tanpa menambah account system, public launch machinery, payment, atau multi-tenancy.

### 16.3 Closed-beta release gate

- [ ] Single-instance web/API/WSS dan PostgreSQL dapat dideploy dan di-rollback.
- [ ] Origin, credential, WS ticket, secret, dan hidden-hand projection direview.
- [ ] Restart di tengah board dan match pulih dari committed revision.
- [ ] Smoke test memakai jumlah meja yang benar-benar direncanakan untuk closed beta, plus margin kecil.
- [ ] Known limitations, WBF compliance boundary, dan contact operasional tersedia bagi tester.
- [ ] Tidak ada Redis, distributed actor, Pub/Sub, multi-instance, public SLO, payment, atau account provider.

---

## 17. Delivery roadmap phase-by-phase

Estimasi mengasumsikan satu engineer full-time yang berpengalaman, requirement cepat diputuskan, dan UI visual tidak membutuhkan branding kompleks. Range adalah engineering days, bukan janji kalender.

| Phase | Hasil | Estimasi |
|---|---|---:|
| 0 | keputusan produk/domain/security terkunci | 2–3 hari |
| 1 | repository, contracts, CI, staging foundation | 3–5 hari |
| 2 | pure bridge engine teruji | 6–9 hari |
| 3 | stable single-table server dan smooth web gameplay | 10–15 hari |
| 3A / GUX | Gameplay UX Reliability & Interaction Refactor | estimasi setelah regression baseline |
| 4 | WBF boundary, deal sources, dan DDS analysis | 6–9 hari |
| 5 | internal Team Match, IMP, dan closed beta | 8–12 hari |
| **Existing target subtotal** | **Phase 0–5 sebelum estimasi GUX** | **35–48 engineering days + GUX** |

Phase 3–7 lama dipadatkan menjadi tiga phase implementasi. Tidak ada phase public community atau scale-out. Estimasi hanya alat urut kerja; stability dan correctness lebih penting daripada tanggal.

### Phase 0 — Product contract dan risk closure (2–3 hari) — COMPLETE

**Objective:** menghilangkan ambiguity yang dapat memaksa rewrite engine, auth, atau table lifecycle.

Work:

- Verifikasi decision register section 20 dan turunkan menjadi [product/rules acceptance contract](docs/product/product-contract.md).
- Review aturan duplicate scoring terhadap sumber normative WBF; independent experienced-player sign-off menjadi assurance pra-rilis Phase 5.
- Buat [threat model dan data classification](docs/security/threat-model.md).
- Rekam [ADR-001 sampai ADR-005](docs/README.md#phase-0-artifacts): modular monolith, PostgreSQL authoritative, WS protocol, guest auth/ticket, dan no Redis MVP.
- Buat [wireflow low-fidelity](docs/product/wireflows.md): landing, join, lobby, table, reconnect, result, cold start, expiry.
- Tetapkan [telemetry dan product events](docs/observability/telemetry-contract.md) tanpa payload private.

**Objective of Done / exit gate:**

- [x] Semua keputusan final section 20 tercermin di product spec, contracts, dan acceptance tests.
- [x] State machine, permissions, scoring rules, retention, recovery, dan error behavior sudah direview.
- [x] Threat model memiliki owner/mitigation/test untuk setiap high risk.
- [x] Tidak ada fitur Phase 1 yang masih memerlukan pilihan arsitektur besar.

Evidence dan decision traceability: [`docs/README.md`](docs/README.md#phase-0-exit-report).

### Phase 1 — Repository dan platform foundation (3–5 hari)

**Objective:** setiap perubahan berikutnya dibangun di atas local/CI/deploy path yang repeatable.

Work:

- Bootstrap monorepo, pinned Go/Node/package manager, strict TypeScript.
- Supabase PostgreSQL untuk runtime owner, PostgreSQL ephemeral untuk CI, config validation, dan secret template.
- Health/live/ready, structured logger, correlation ID, panic recovery.
- Migration + sqlc/pgx setup.
- OpenAPI/WS schema skeleton dan code generation.
- CI lint/test/build/security scan; protected main rules.
- Jalankan web/API/TLS/WSS probe melalui Compose lokal; verify WSS echo hanya pada probe terisolasi, bukan API produk.

**Objective of Done / exit gate:**

- [x] Fresh clone + secret lokal satu kali → satu documented command → services + migration berjalan.
- [x] CI hijau dan gagal bila generated contract/migration tidak sinkron.
- [x] HTTPS/WSS lokal terhubung dengan exact Origin allowlist dan TLS verification aktif.
- [x] Config invalid fail-fast tanpa mencetak secret.
- [x] Deploy/rollback smoke runbook berhasil sekali.

**Status implementasi 30 Agustus 2026:** Phase 1 selesai pada boundary lokal + Supabase. GitHub CI run `33265699414` hijau, required check `quality` dan branch protection aktif. CI membuktikan full baseline → candidate → rollback → promotion memakai PostgreSQL ephemeral, HTTPS, exact Origin, dan WSS probe. Runtime owner memakai Supabase melalui `.env.local-gate`; tidak ada Render, Vercel, Redis, payment, atau SaaS deployment lain. Runbook tersedia di `docs/runbooks/phase-1-deploy.md` dan sengaja tidak dilacak Git sesuai kebijakan repository.

### Phase 2 — Pure bridge game engine (6–9 hari) — COMPLETE

**Objective:** seluruh aturan game dapat dibuktikan tanpa browser, socket, atau database.

Work:

- Card/deck/seat/partnership/value objects.
- CSPRNG deal interface, dealer/vulnerability cycle.
- Auction legality dan final contract/declarer.
- Play legality, follow suit, dummy control, trick winner.
- Duplicate score engine + ruleset version.
- Pure decision/reducer/invariant validation.
- Fixture serializer untuk E2E deterministic test (tidak diaktifkan di production).
- Unit/property/fuzz/golden tests.

**Objective of Done / exit gate:**

- [x] Semua invariant section 8 memiliki test positif dan negatif.
- [x] Golden score matrix sudah diverifikasi terhadap WBF Laws of Duplicate Bridge 2017 Law 77, termasuk boundary kontrak, doubled, redoubled, vulnerability, overtrick, undertrick, game, slam, dan insult bonus.
- [x] Randomized legal games 10.000 run selesai tanpa invariant violation, termasuk dengan race detector.
- [x] Fuzz card decoder, decision engine, dan test-only fixture decoder tidak menemukan panic pada budget CI yang ditetapkan.
- [x] Package engine tidak mengimpor network, DB, framework, atau global clock/RNG; guard test menegakkan boundary ini.

**Status implementasi 30 Agustus 2026:** Phase 2 selesai. Engine murni mencakup deal deterministik melalui RNG terinjeksi, cycle dealer/vulnerability, auction lengkap, declarer/opening lead, dummy control, follow-suit, 13 trick, duplicate scoring berversi, replay reducer, invariant validation, serta fixture khusus test build. Verifikasi phase gate lulus pada race suite, 10.000 randomized boards, golden matrix berbasis WBF Law 77, fuzz targets, lint/vet, vulnerability scan, migration validation, Supabase integration test, dan statement coverage engine 90,1% (fixture 90,2%).

**Batas WBF Phase 2:** status COMPLETE hanya membuktikan mekanik engine yang sudah disebut dan duplicate scoring Law 77. Status ini tidak mengklaim seluruh Laws of Duplicate Bridge sudah diimplementasikan. Phase 4 membuat compliance matrix untuk setiap law relevan, menambah enforcement mekanis yang kurang tanpa melanggar pure-engine boundary, serta mengklasifikasikan law yang memerlukan Director atau tidak berlaku pada format online ini.

**Standar test lanjutan:** perubahan kecil wajib memakai kombinasi test kecil dan terarah pada package/skenario yang berubah. Randomized suite default dibatasi 250 game. Run 10.000 game hanya dijalankan eksplisit melalui `make test-engine-stress` pada milestone, investigasi khusus, atau sebelum release; bukan pada setiap perubahan maupun default PR gate. Full `make gate-engine` digunakan pada penutupan phase atau perubahan lintas komponen.

### Phase 3 — Stable single-table server dan smooth gameplay (10–15 hari)

> **P0 web follow-up (planning only):** guest → lobby routing, typed/actionable error UX, controller takeover recovery, dan complete BBO-structured table refactor dirinci di `apps/web/PLAN.md`. Roadmap tersebut harus diselesaikan sebelum four-browser Phase 3 UX gate dianggap lulus; tidak mengubah pure engine atau realtime architecture.

**Objective:** empat guest dapat berpindah dari room ke table, menuntaskan board melalui WebSocket, dan pulih dari disconnect/restart tanpa kehilangan atau membocorkan state.

**Work:**

- Selesaikan guest credential, invite, participant, room/table/seat, ready/start/leave/finish, dan owner controls minimum.
- Persist event + private snapshot secara transaksional; tegakkan monotonic revision, sequence, request idempotency, dan commit-before-broadcast.
- Gunakan satu local table actor per table dengan bounded queue, hydrate/evict, graceful drain, dan tanpa distributed abstraction.
- Implementasikan WS ticket, origin/auth per message, ack/error/event, recipient projector, reconnect/resume, event-gap recovery, snapshot fallback, dan controller fencing.
- Implementasikan web lobby/table untuk auction, play, dummy, trick, result, next board, connection/degraded state, dan input keyboard/mobile yang diperlukan.
- Jadikan room-to-room/table transition eksplisit: unsubscribe lama, release/retain seat sesuai command, clear private client state, subscribe baru, dan reject stale command dari room sebelumnya.
- Tambahkan log/metrics minimum untuk command, revision, reconnect, queue, DB, dan panic.

**Explicit non-goals:**

- DDS, Team Match, configurable deal source selain secure random dan fixture test yang sudah ada.
- Chat, spectator, replay publik, moderation console, public profile, OAuth/passkey, analytics funnel, dan public launch work.
- Redis, distributed actor, Pub/Sub, multi-instance, load target internet-scale, dan zero-downtime deployment.

**Objective of Done:**

- [ ] Four scripted clients dan empat browser context menuntaskan create → join → seat → auction → play → score → next/finish.
- [ ] Disconnect, duplicate/out-of-order command, missed event, snapshot fallback, multi-tab takeover, backend restart, dan table switch pulih ke committed revision.
- [ ] Commit-before-broadcast dan idempotency terbukti; rejected action tidak mengubah state.
- [ ] Projection matrix dan network assertion membuktikan tidak ada hidden-hand leakage pada snapshot, event, error, result sebelum reveal, atau logs.
- [ ] Tidak ada stale seat/controller/subscription/private state setelah room-to-room transition.
- [ ] Race detector, goroutine leak check, contract validation, dan smoke test closed-beta kecil lulus.

**Cheapest appropriate test level:** pure unit test untuk projector/state transition; PostgreSQL integration test untuk transaction, revision, idempotency, hydrate; scripted WS contract test untuk ordering/reconnect; satu Playwright four-context happy path untuk wiring UI penuh. Jangan memakai E2E untuk kasus yang dapat dibuktikan di unit/integration.

**Status implementasi 30 Agustus 2026:** Work 1–5 selesai. Guest/table lifecycle, durable command transaction, process-local table actor, transport WebSocket v1, dan web lobby/table sudah tersedia. Web mencakup guest session restore, create/join via invite, seat/ready/owner controls, auction keyboard/mobile controls, follow-suit card input, declarer dummy control, current trick, result, next board/finish, explicit connected/degraded/offline state, resumable sequence, dan pembersihan private/pending state ketika koneksi atau meja berubah. Reducer unit test, lint, typecheck, dan production build web lulus; scripted WS contract, recipient privacy, package race, serta PostgreSQL command/resume integration test dari Work 4 tetap lulus. Phase 3 tetap terbuka; Work 6 berikutnya mengeraskan room/table transition dan menutup four-browser persisted/reconnect/privacy gate sebelum observability minimum.

### Objective GUX — Gameplay UX Reliability & Interaction Refactor — IN PROGRESS / UX-01–UX-13 PASS

**Objective:** membuat aksi bridge terasa langsung, predictable, spatially continuous, dan usable di desktop/mobile tanpa decorative redesign atau perubahan prematur pada pure engine. UX correctness dan gameplay feel mempunyai prioritas kira-kira 10× decorative quality.

Canonical detail, repository audit, observable acceptance, browser matrix, dan evidence ledger berada di [`apps/web/PLAN.md`](apps/web/PLAN.md#19-objective-gux--gameplay-ux-reliability--interaction-refactor). Objective ini frontend-first, regression-sensitive, dan bergantung pada realtime core/revision fencing/recipient projection serta pure bridge engine yang sudah ada.

Sub-objective wajib:

1. UX-01 frontend component architecture dan one canonical card primitive.
2. UX-02 authoritative/optimistic/presentation state separation dan reconciliation.
3. UX-03 prevention untuk known-illegal actions.
4. UX-04 central gameplay animation/presentation queue dan board-click skip.
5. UX-05 shared click/tap/Pointer Events drag command path.
6. UX-06 shared card scale dan explicit non-overlapping board geometry.
7. UX-07 concise contract + declaring seat/identity.
8. UX-08 Claim/Undo di navbar dengan capability state.
9. UX-09 concise active-game copy.
10. UX-10 transition-based auction/play turn cue dan mute.
11. UX-11 viewer-relative upright won/sideways lost trick indicator.
12. UX-12 selectable invite code tanpa required clipboard button.
13. UX-13 right-side robot icon pada participant identity.
14. UX-14 automated cross-breakpoint gameplay verification.

Required order adalah audit baseline → behavior-preserving UX-01 extraction → UX-02/03 interaction state → UX-06/07/08/09/11/12/13 presentation → UX-04/05/10 motion/physical input → UX-14 verification.

#### GATE UX-G1 — Gameplay UX Refactor Complete

- [x] UX-01 PASS
- [x] UX-02 PASS
- [x] UX-03 PASS
- [x] UX-04 PASS
- [x] UX-05 PASS
- [x] UX-06 PASS
- [x] UX-07 PASS
- [x] UX-08 PASS
- [x] UX-09 PASS
- [x] UX-10 PASS
- [x] UX-11 PASS
- [x] UX-12 PASS
- [x] UX-13 PASS
- [ ] UX-14 PASS

ENG-01 Play History, ENG-02 Table Score Sheet, dan ENG-03 Bot Consensus Behavior berstatus **BLOCKED BY UX-G1**. Jangan interleave ketiganya dengan GUX. Frontend-discovered engine limitation dicatat sebagai dependency; engine exception sebelum gate memerlukan bukti frontend contract benar-benar mustahil, smallest compatible change, ADR amendment, dan explicit roadmap approval. Existing Phase 4 work yang bukan ENG-01/02/03 tidak otomatis dibatalkan, tetapi tidak boleh disisipkan ke commit GUX atau dipakai untuk melewati gate ini.

### Phase 4 — WBF boundary, deal sources, dan DDS analysis (6–9 hari)

**Objective:** menutup correctness rules/deal/analysis untuk duplicate bridge tanpa memasukkan infrastructure atau DDS ke pure engine.

**Work:**

- Buat WBF compliance matrix per law yang relevan dengan status `mechanically-enforced`, `director-judgement`, atau `not-applicable`, disertai alasan dan test/evidence.
- Tambahkan enforcement untuk setiap tindakan mekanis yang masih dapat dicegah server; illegal call/play/turn/controller action tidak pernah menjadi irregularity tersimpan.
- Dokumentasikan terutama boundary Director: rectification/adjusted score, disputed facts, tempo/unauthorized information/conduct, disclosure/convention judgment, dan tournament procedure.
- Tambahkan application-level deal-source interface dengan implementasi CSPRNG production, deterministic/test, prepared deal, dan configurable distribution/constraint source yang benar-benar diperlukan.
- Persist provenance, stable board identity, dealer, vulnerability, source type/version, dan safe source reference; jangan log seed atau hand.
- Integrasikan DDS melalui analysis API/domain boundary untuk double-dummy table, makeable contracts, dan par result. Input DDS adalah deal value/DTO, bukan mutable engine state; failure DDS tidak mengubah board/result.

**Explicit non-goals:**

- Mengklaim otomatisasi Director, adjusted score, appeals, ethics/conduct, convention ontology, problem classification, bidding hints, atau AI review.
- Memasukkan DDS, filesystem/process/network dependency, atau deal-source orchestration ke pure engine.
- General-purpose plugin system untuk deal generator atau solver; hanya source yang diminta di atas.

**Objective of Done:**

- [ ] Setiap WBF law relevan mempunyai klasifikasi, alasan, dan evidence; reviewer bridge berpengalaman menyetujui boundary.
- [ ] Semua law mekanis dalam matrix mempunyai positive/negative test dan illegal action ditolak tanpa revision change.
- [ ] Keempat deal source menghasilkan 52 kartu valid dan provenance yang dapat dibaca kembali; prepared/constraint failure bersifat eksplisit dan tidak membuat board parsial.
- [ ] Board identity, dealer, dan vulnerability tidak berubah ketika deal dipakai ulang pada context yang mengharuskannya.
- [ ] DDS golden fixtures memverifikasi double-dummy table, makeable contracts, dan par result terhadap output solver yang dipin.
- [ ] Guard test membuktikan package pure engine tidak mengimpor DDS atau application/infrastructure packages.

**Cheapest appropriate test level:** table-driven/unit test untuk law matrix mapping, validators, IMP-independent deal invariants, dan DDS result mapping; golden/component test terhadap pinned DDS untuk sedikit fixture representatif; PostgreSQL integration test hanya untuk provenance; satu API contract test untuk analysis. Tidak perlu browser E2E kecuali satu smoke display hasil analysis.

**Status implementasi 31 Agustus 2026:** Work 1–2 selesai. Matriks Law 1–93 memakai hanya status `mechanically-enforced`, `director-judgement`, atau `not-applicable`, memuat rationale dan executable evidence, serta mengikuti revisi WBF Laws 73/89 efektif 1 Januari 2024. Mechanical command boundary menolak out-of-turn call/lead, insufficient atau inadmissible call, wrong-hand play, dummy self-play, revoke attempt, dan mixed game payload tanpa mengubah aggregate/revision/sequence. Independent experienced-player approval tetap pending sebagai exit gate. Work 3 (deal-source interface) dan seluruh pekerjaan sesudahnya belum dimulai.

### Phase 5 — Internal Team Match dan closed beta (8–12 hari)

**Objective:** dua tim memainkan board identik pada dua meja, memperoleh IMP dan total match durabel, lalu produk siap dipakai closed beta kecil.

**Work:**

- Implementasikan lifecycle create match, assign dua tim, create/manage exactly two required tables/rooms, seat assignment, start, active, complete, dan minimal owner recovery/control.
- Buat board set sekali per match lalu referensikan board identity yang sama di kedua room; pertahankan deal, board number, dealer, vulnerability, dan provenance.
- Sinkronkan availability/progression board pada kedua meja tanpa mengunci gameplay satu meja pada latency meja lain.
- Collect exactly one final result per board/table, bandingkan score dari perspektif tim yang konsisten, hitung IMP dengan WBF Law 78B scale, akumulasikan total, dan finalize secara idempotent.
- Persist match state, paired board results, IMP, total, dan final result; restart/retry tidak boleh menggandakan comparison.
- Tambahkan UI minimum untuk create match, assign teams/seats, masuk ke room yang benar, melihat readiness/progress, dan final score.
- Jalankan focused hardening: restart/reconnect, DB failure, hidden-data review antar room, dependency/secret scan, rollback aplikasi, dan pilot closed beta kecil.

**Explicit non-goals:**

- Tournament movement, pair event, matchpoints, Swiss/round-robin bracket, leaderboard, public match discovery, Director console, atau community roles.
- Spectator, chat, anti-collusion detection, cross-table presence, atau result sharing publik. Pencegahan hidden-hand leakage antar kedua room tetap wajib.
- Public community launch, OAuth/passkey, profile/history lintas device, payment, commercial architecture, multi-tenancy, Redis, multi-instance, public SLO, advanced disaster recovery, dan elaborate retention.

**Objective of Done:**

- [ ] Dua tim/eight seats dapat membuat match, masuk ke dua room yang benar, dan menuntaskan board set tanpa operasi database/manual admin.
- [ ] Kedua room menerima board identity/deal/dealer/vulnerability yang sama dengan partnership orientation yang benar, tanpa menerima hidden state room lain.
- [ ] Golden WBF IMP boundary cases, sign/orientation, tie, passed-out, dan aggregate total benar.
- [ ] Duplicate result delivery/retry/restart menerapkan comparison tepat satu kali dan final result persisted dapat dihydrate.
- [ ] Satu meja dapat maju sesuai policy tanpa race atau corrupt state ketika meja pasangannya lebih lambat/disconnect.
- [ ] Closed-beta smoke pada jumlah meja yang direncanakan, restart di tengah match, security/privacy checks, dan seluruh Objective of Done section 3.2 lulus.
- [ ] Known limitations dan WBF compliance boundary terlihat bagi tester; tidak ada unresolved correctness, data-loss, hidden-hand, atau security severity tinggi.

**Cheapest appropriate test level:** pure table-driven/golden test untuk IMP dan orientation; application unit test untuk match state machine; PostgreSQL integration test untuk paired-result idempotency/finalization/restart; scripted eight-client test untuk board synchronization dan privacy; satu Playwright Team Match happy path sebagai release smoke.

### Deferred outside this roadmap

Public community ecosystem, chat, spectator, moderation console, OAuth/passkey, public profiles, complex analytics, payments, commercial/multi-tenant architecture, Redis/scale-out, distributed actors, Pub/Sub, multi-instance, public SLO machinery, advanced disaster recovery, elaborate retention, tournament movement selain internal Team Match, community roles, dan anti-collusion systems tidak memiliki phase atau speculative extension point. Membuka salah satunya memerlukan product decision dan roadmap baru setelah target product di atas stabil.

---

## 18. Prioritas backlog/urutan implementasi

Di dalam GUX, jangan mulai dari animasi kartu: lock regression baseline, extract component boundaries, lalu selesaikan optimistic/capability state lebih dahulu. Critical path implementation:

```text
P0 validate locked decisions
→ repo/CI/contracts/config
→ pure cards/deal
→ auction
→ play/trick
→ scoring
→ DB event+snapshot transaction
→ guest/table/seat lifecycle
→ projector privacy
→ WS actor/ack/event
→ reconnect/idempotency/fencing
→ four-client scripted test
→ web lobby/auction/play/result
→ room-transition hardening
→ GUX audit + behavior-preserving component extraction
→ optimistic reconciliation + known-illegal prevention
→ canonical card/board presentation
→ gameplay motion + pointer drag + turn audio
→ cross-breakpoint UX-G1 gate
→ WBF compliance matrix + missing mechanical enforcement
→ deal sources + provenance
→ DDS analysis boundary
→ Team Match board synchronization
→ IMP + persisted match result
→ focused security/restart/smoke
→ closed beta
```

Sesudah UX-G1 PASS, branch engine GUX berjalan berurutan `ENG-01 play history → ENG-02 score-sheet semantics/implementation → ENG-03 bot consensus`, dengan exception ordering hanya bila dependency terdokumentasi di `apps/web/PLAN.md`.

Suggested epics:

| Epic | Deliverable utama | Dependency |
|---|---|---|
| E0 Locked decisions & ADR | product contract | none |
| E1 Platform | runnable monorepo + CI + contracts | E0 |
| E2 Bridge engine | pure tested engine | E0–E1 |
| E3 Durable aggregate | schema/event/snapshot/idempotency | E1–E2 |
| E4 Identity/table | guest/invite/seat permissions | E1, E3 |
| E5 Realtime | actor/protocol/projection/recovery | E2–E4 |
| E6 Web gameplay | smooth single-table user journey dan room transition | E4–E5 |
| E6A GUX frontend | reliable optimistic interaction, physical presentation, cross-breakpoint gate | E5–E6 |
| E6B GUX engine extensions | history, approved score sheet, deterministic bot consensus | E6A / UX-G1 |
| E7 WBF compliance | law matrix + missing mechanical enforcement | E2, E5 |
| E8 Deal & analysis | source provenance + DDS boundary | E3, E7 |
| E9 Team Match | synchronized boards + IMP + persisted match result | E3–E8 |
| E10 Closed beta | focused privacy/restart/security/smoke evidence | all prior |

---

## 19. Risk register

| Risk | Probability/impact | Mitigation | Early signal |
|---|---|---|---|
| Hidden hand bocor | medium/critical | single projector, privacy matrix, response inspection, no raw logs | unexpected payload size/card in client |
| Rule/scoring salah | medium/high | pure engine, golden/property tests, domain review, ruleset version | dispute/manual mismatch |
| Restart menghilangkan state | medium/high | DB commit-before-broadcast, snapshot/event, focused restart test | hydrate mismatch/revision gap |
| Race seat/command | high/high | actor + DB revision + unique constraints + idempotency | conflict/duplicate metric |
| Room transition meninggalkan authority/state | medium/high | explicit unsubscribe/release/clear/fence state machine | stale event atau seat setelah switch |
| Optimistic projection drift dari authoritative state | medium/critical | operation identity + base revision, deterministic rebase, snapshot fallback, delayed/rejected E2E | duplicate card/call atau hand/trick tidak cocok sesudah ACK |
| Animation queue mengaburkan state atau mengubah ordering | medium/high | pisahkan logical/presentation state, central queue, board-only skip, burst-event tests | legality menunggu timer atau trick hilang/terurut salah |
| Board geometry collision pada dummy/mobile | high/high | explicit zones + shared card scale + bounding-box matrix 320px–desktop | dummy/trick/playable hand overlap atau rank terpotong |
| History membocorkan completed tricks | medium/critical | server-side entitlement projection dan raw-frame privacy matrix | non-Dummy menerima history lebih luas dari policy |
| Istilah IMP score sheet tidak mempunyai comparison source | high/high | block ENG-02 sampai semantics/reference/pair lifecycle disetujui | UI melabel raw duplicate score sebagai IMP |
| Bot consensus tidak pernah terminal | medium/high | durable actor-owned transition table dan exhaustive bot/human tests | `actionRequest` tersisa tanpa responder yang mungkin |
| Board pasangan Team Match berbeda | medium/critical | shared board identity + immutable deal/provenance | dealer/vulnerability/deal mismatch |
| IMP diterapkan dua kali atau orientasi salah | medium/high | unique comparison + idempotent finalization + golden boundaries | total berubah setelah retry |
| DDS mencemari engine atau mengubah result | low/high | analysis boundary + dependency guard + read-only request | engine import/failure mutates board |
| Deal source menghasilkan board ambigu | medium/high | validation + persisted provenance + atomic board creation | missing provenance/partial board |
| Invite brute force/abuse | low-medium/high | entropy, uniform errors, per-instance rate limit | join failures spike |
| Reconnect storm | medium/high | backoff+jitter, ticket rate, bounded queues | simultaneous handshake surge |
| Scope melebar ke platform/tournament | high/high | explicit deferred list and phase gates | account/chat/scale-out work masuk sprint |
| Guest kehilangan seat | medium/medium | durable device identity, grace, recovery policy | support request/multi-device |

---

## 20. Decision register — final dan closed

Semua decision berikut berlaku sebagai baseline implementasi. Phase 0 tidak memilih ulang; tugasnya menurunkan keputusan ini menjadi ADR, contract, migration, UI behavior, dan acceptance test. Perubahan hanya melalui product change/ADR baru.

| ID | Keputusan final | Konsekuensi/acceptance behavior |
|---|---|---|
| OD-01 | Satu table dapat memainkan board berikutnya berulang kali sampai owner memilih finish pada state `BETWEEN_BOARDS`; tidak ada tournament movement pada MVP. | `next_board` idempotent, hanya di safe state, board number naik satu, dan table tidak dapat dihidupkan kembali setelah finished. |
| OD-02 | Presence memakai jumlah subscribed connection per participant. WebSocket disconnect hanya mengubah presence menjadi offline dan tidak melepaskan participant atau seat. | Marker browser hanya kandidat recovery; membership server menentukan `TABLE_ACTIVE` atau `TABLE_EXPIRED`. Lihat ADR-011. |
| OD-03 | Waiting table expire setelah **2 jam** tanpa meaningful activity; active table menjadi abandoned setelah **24 jam** tanpa meaningful activity; completed result hanya untuk participant dan disimpan **90 hari**. | Heartbeat/presence tidak memperpanjang TTL; warning dikirim sebelum expiry; cleanup idempotent. |
| OD-04 | Owner disconnect tidak memindahkan ownership. Transfer atau removal hanya terjadi melalui command lifecycle eksplisit dan owner terakhir tidak dapat dilepas tanpa pengganti. | Presence transport tidak menentukan membership atau ownership. Lihat ADR-011. |
| OD-05 | Full deal hanya terlihat oleh participant meja setelah board berstatus completed; tidak terlihat oleh visitor, invite preview, atau public spectator. | Semua result tetap melewati projector dan `Cache-Control: private, no-store`. |
| OD-06 | Dealer dan vulnerability mengikuti standard **16-board duplicate cycle**, kemudian mengulang untuk board 17+. | Board number, dealer, dan vulnerability dihitung server-side dan dilindungi golden tests. |
| OD-07 | Nilai canonical adalah signed integer `score_ns`; score EW selalu `-score_ns`. UI menampilkan nilai dari perspektif partnership viewer. | Database/API tidak menyimpan dua score independen yang bisa berbeda. |
| OD-08 | UI memakai **React Aria Components** untuk accessible interaction primitives dan **Tailwind CSS** untuk styling/design tokens. | Tidak menambahkan design system kedua; custom card interaction tetap memenuhi keyboard/screen-reader path. |
| OD-09 | Access token short-lived disimpan hanya di memory; rotatable opaque device credential disimpan di IndexedDB dan ditukar melalui HTTPS. WS memakai single-use ticket TTL 30–60 detik. | CSP ketat, exact Origin, credential hash-at-rest, rotation/revoke, dan no token in analytics/log; browser tanpa storage mendapat warning recovery terbatas. |
| OD-10 | Backend memakai Go `net/http` + `go-chi/chi/v5`; WebSocket memakai `github.com/coder/websocket`. Gin tidak digunakan. | Dependency dipin; boundary domain tidak bergantung pada router/transport. |
| OD-11 | Tidak memakai third-party product analytics, session replay, atau analytics funnel kompleks. | Operasi hanya mengumpulkan logs/metrics minimum; tidak ada raw hand, token, invite, nickname, atau WS payload di telemetry. |
| OD-12 | Tidak ada payment, subscription, billing, paid plan, premium feature, iklan, atau arsitektur komersial dalam roadmap ini. | Jangan membuat provider/SDK/table/entitlement/quota abstraction untuk monetisasi atau public community launch. |
| OD-13 | Deployment target adalah single instance untuk closed beta kecil. | Redis, distributed actor, Pub/Sub, multi-instance, public SLO, dan scale-out tidak diimplementasikan secara spekulatif. |
| OD-14 | Internal Team Match dua meja adalah satu-satunya format lintas meja. | Board identity/deal/dealer/vulnerability sama di kedua room; result dibandingkan idempotently dengan WBF Law 78B IMP scale. |
| OD-15 | DDS hanya tersedia melalui analysis boundary di luar pure engine. | DDS failure tidak mengubah authoritative board atau result; dependency guard melarang import ke engine. |
| OD-16 | Setiap board mempunyai deal-source provenance. | Mendukung secure random, deterministic test, prepared, dan configurable constraint source tanpa ontology/problem classification. |
| OD-17 | Owner dapat mengelola bot kursi sederhana. | Bot bukan guest identity; add/remove/replace bersifat durabel, bot memilih legal call/card pertama melalui actor, dan claim/undo dinonaktifkan saat bot duduk. Lihat ADR-008. |
| OD-18 | Gameplay UX reliability memakai server-authoritative optimistic client, logical/presentation separation, one canonical card primitive, one play-command path, dan UX-G1 engine gate. | Local legal call/play terlihat segera lalu reconcile/rollback dengan revision/request identity; known-illegal action dicegah; motion terurut dapat di-skip hanya lewat board; ENG-01/02/03 blocked sampai UX-01–14 PASS. Lihat ADR-009 dan `apps/web/PLAN.md`. |
| OD-19 | Connection web terbaru untuk guest yang sudah duduk otomatis mengambil controller setelah projection fresh. | Tidak ada konfirmasi takeover; fencing epoch tetap tunggal, device lama ditolak `STALE_CONTROLLER`, dan mutation user yang ditolak tidak diulang otomatis. Lihat ADR-010. |

Status OD-01 sampai OD-12: **CLOSED — accepted 29 Agustus 2026**. OD-13 sampai OD-16 mencatat refactor scope 30 Agustus 2026. OD-17 diterima 31 Agustus 2026 sebagai perubahan produk eksplisit dan menjadi baseline Phase 3–5. OD-18 diterima 1 September 2026 sebagai architecture/sequence baseline Objective GUX; implementation sedang berjalan dengan UX-01–UX-13 PASS, sedangkan UX-14 belum dimulai. OD-19 diterima 2 September 2026 dan menggantikan konfirmasi takeover manual dengan handoff otomatis pada connection terbaru.

---

## 21. Evidence checklist untuk release MVP

Isi link commit/test/dashboard pada saat implementasi:

```text
[ ] Domain reviewer sign-off: ____________________
[ ] Engine golden/property report: _______________
[ ] WBF compliance matrix/reviewer: ______________
[ ] Projection/privacy test report: ______________
[ ] Four-browser E2E run: _________________________
[ ] Deal provenance/source report: _______________
[ ] DDS golden/API report: ________________________
[ ] Eight-client Team Match run: __________________
[ ] IMP golden/persistence report: ________________
[ ] Restart/recovery drill: _______________________
[ ] Closed-beta smoke report: _____________________
[ ] Security scan/threat review: __________________
[ ] Accessibility review: _________________________
[ ] Deploy/rollback runbook execution: ____________
[ ] Known limitations published: _________________
```

---

## 22. Referensi keputusan

Referensi domain dan technical baseline:

- [Supabase pricing](https://supabase.com/pricing) dan [PostgreSQL connection guide](https://supabase.com/docs/guides/database/connecting-to-postgres) — free limits dan pooling.
- [OWASP WebSocket Security Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/WebSocket_Security_Cheat_Sheet.html) — WSS, Origin, authz per message, input/DoS limits, logging.
- [Coder WebSocket for Go](https://github.com/coder/websocket) — candidate library realtime Go.
- [WBF Laws of Duplicate Bridge](https://www.worldbridge.org/regulations/2017-laws-of-duplicate-bridge/) — sumber normative; versi resmi saat ini mencakup revisi Laws 73 dan 89 efektif 1 Januari 2024.
- [WCAG 2.2](https://www.w3.org/TR/WCAG22/) — target accessibility critical path.

---

## 23. Langkah implementasi berikutnya

1. Tutup regression baseline Phase 3 yang masih pending, lalu mulai Objective GUX Stage A; jangan mulai implementasi dari motion atau decorative polish.
2. Jalankan GUX sesuai urutan `UX-01 → UX-02/03 → UX-06/07/08/09/11/12/13 → UX-04/05/10 → UX-14`, kemudian evaluasi UX-G1.
3. Jangan mulai ENG-01 Play History, ENG-02 Table Score Sheet, atau ENG-03 Bot Consensus Behavior sebelum UX-G1 PASS.
4. Jangan mulai DDS atau Team Match sebelum four-client persisted/reconnect/privacy gate Phase 3 lulus; integrasikan deal sources/DDS di luar pure engine.
5. Bangun Team Match dari stable board identity dan paired results, lalu tutup dengan IMP/persisted finalization dan focused closed-beta evidence.
6. Tolak backlog platform/community/scale-out sampai roadmap baru secara eksplisit mengotorisasinya.
