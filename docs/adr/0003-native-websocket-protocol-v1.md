# ADR-003: Native WebSocket Protocol v1

- Status: Accepted
- Date: 29 Agustus 2026
- Last amended: 2 September 2026
- Decision owners: Engineering
- Supersedes: none

## Context

Bridge memerlukan bidirectional low-latency commands, ordered server events, acknowledgments, presence, reconnect/resync, dan long-lived connections. SSE hanya server-to-client; polling menambah latency/overhead. Socket.IO memberi conveniences tetapi menambah protocol/runtime dependency yang tidak diperlukan untuk browser modern dan Go backend.

## Decision

Gunakan native RFC 6455 WebSocket melalui `github.com/coder/websocket`, endpoint `/v1/ws`, dengan versioned JSON text protocol.

REST tetap digunakan untuk guest/session, create/join lifecycle, WS ticket, history/result, health, dan abuse report. WSS digunakan untuk subscribe/resume, presence, lobby mutations yang realtime, game commands, ack, dan ordered events.

### Envelope

Common fields:

- `v`: protocol major, MVP `1`;
- `kind`: `command`, `ack`, `event`, `error`, `snapshot`, `control`;
- `name`: bounded message name;
- `request_id`: client-generated idempotency ID pada command;
- `table_id`;
- `expected_revision` pada mutation;
- `revision` dan `seq` pada accepted server state/event;
- typed `payload` validated by JSON Schema.

### Connection model

- Satu reader dan satu bounded writer loop per connection.
- Table subscription bertindak sebagai room scope; authorization terjadi saat subscribe dan setiap mutation.
- Satu table actor memproses command berurutan.
- Satu principal dapat mempunyai beberapa connection, tetapi web client terbaru otomatis meminta controller epoch baru setelah projection fresh sesuai ADR-010; hanya satu controller epoch aktif per seat.
- Server protocol ping/pong mendeteksi dead peer.
- Browser mengirim jittered application heartbeat selama realtime screen aktif; heartbeat tidak menyentuh DB/Redis.
- Text JSON only, max 8 KiB, compression off.

### Ack, ordering, dan reconnect

- Accepted ack membawa request ID, revision, dan seq.
- Domain/security error tidak mengubah revision.
- Duplicate request ID mengembalikan prior outcome tanpa second mutation.
- Client mengabaikan `seq <= last_seen_seq`.
- Gap `seq > last_seen_seq + 1` menghentikan mutation dan memicu resync.
- Resume mengirim `last_seen_seq`; server memberi bounded missing events atau latest snapshot.
- Reconnect menggunakan exponential backoff + full jitter, cap 30 detik.
- Client tidak mengantre arbitrary unsent game commands ketika offline karena intent dapat stale. Hanya already-sent idempotent command yang boleh di-retry dengan request ID sama setelah outcome tidak diketahui.

### Optimistic client correlation

ADR-009 mengizinkan client memproyeksikan call/play lokal yang diketahui legal tanpa mengubah authority protocol ini.

- Optimistic operation wajib menyimpan `request_id`, command/payload, base revision, dan expected presentation effect.
- Accepted ACK dengan request ID + revision + seq adalah receipt untuk command, bukan pengganti recipient projection authoritative.
- Ordered projected event/snapshot tetap menjadi source untuk confirm/rebase visible state. Rejection atau revision conflict menghapus operation terkait lalu rebase dari projection authoritative terbaru; snapshot/resume adalah recovery terakhir.
- Client tidak boleh menghapus semua pending operation hanya karena menerima arbitrary event yang tidak terkait.
- Sebelum implementation, contract test harus membuktikan apakah ACK revision/seq dan ordered projected event cukup untuk correlation. Bila ambiguity tetap ada, protocol boleh menambahkan origin command identity pada projected event atau metadata compatible minimum; jangan mencocokkan berdasarkan card/call value saja.
- Logical authoritative/optimistic state dan presentation queue tidak boleh disatukan. Animation tidak menahan seq/revision processing atau menentukan legal action.

Click, tap, dan pointer drop menghasilkan command envelope yang sama melalui satu client action path. Known-illegal action berdasarkan projection tidak dikirim, tetapi server tetap validator final.

### Backpressure

- Outbound queue dibatasi count dan bytes.
- Presence dapat dicoalesce/drop karena reconstructable.
- Domain event tidak di-drop diam-diam; slow consumer ditutup lalu client resync.
- Connection/message capacity dan close reason diukur.

### Close behavior

Gunakan standard close codes bila cocok:

- `1000` normal;
- `1001` page going away;
- `1002` protocol error;
- `1003` unsupported/binary data;
- `1007` invalid payload encoding;
- `1008` auth/policy/rate violation;
- `1009` message too big;
- `1011` unexpected server error;
- `1012` service restart/drain bila library/browser support memungkinkan.

Application error details tetap memakai safe JSON error sebelum close ketika memungkinkan.

## Rationale

- Bidirectional turn commands dan server events berada pada satu connection.
- Native WS meminimalkan protocol overhead/dependency dan cocok dengan Go library yang dipilih.
- Explicit ack/revision/seq menjadikan reconnect dan ordering bagian contract, bukan library magic.
- REST/WSS split menjaga resource lifecycle dan realtime session jelas.

## Consequences

Positive:

- Protocol kecil, typed, versioned, dan mudah dites dari Go/TypeScript.
- Reconnect/resync tidak bergantung reconnect ke instance yang sama.
- Backpressure dan privacy projection dapat dikontrol langsung.

Negative/trade-offs:

- Tim harus mengimplementasi room registry, ack, reconnect, heartbeat, dan backpressure.
- Tidak ada automatic long-polling fallback.
- Browser tidak dapat mengirim protocol Ping frame; perlu application heartbeat untuk inbound activity.

## Alternatives rejected

| Alternative | Alasan ditolak |
|---|---|
| Socket.IO | extra protocol/runtime/adapter complexity; native browser support cukup |
| SSE + HTTP commands | dua transport dan one-way stream; ordering/identity tetap perlu custom handling |
| Long polling | overhead/latency lebih tinggi untuk turn/presence |
| WebRTC | peer-to-peer tidak cocok dengan authoritative hidden-state server |
| GraphQL subscription | schema/runtime overhead tanpa query benefit untuk fixed game protocol |

## Roadmap boundary

- Roadmap saat ini tetap single-instance dan tidak memakai sticky routing, cross-instance fan-out, atau Redis broker.
- Established WebSocket tetap terikat satu gateway dan resume selalu datang dari durable PostgreSQL state.
- Connection cap dinaikkan hanya setelah bounded load test pada package dan boundary yang terdampak.

## Validation

- JSON Schema contract tests untuk semua message names/examples.
- Autobahn/library compatibility dan explicit upgrade handling test.
- Origin/auth rejection, close code, size/rate, malformed/binary frame tests.
- Ack-loss, duplicate, gap, reconnect, slow consumer, and SIGTERM E2E.
- Privacy test memeriksa raw socket payload setiap viewer, bukan hanya DOM.
