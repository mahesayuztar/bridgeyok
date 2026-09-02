# ADR-011: Durable Participant and Session Recovery

- Status: Accepted
- Date: 3 September 2026
- Decision owners: Product/Engineering
- Supersedes: automatic disconnect expiry pada ADR-006 dan recovery routing lama

## Context

WebSocket dapat terputus karena reload, browser sleep, pergantian jaringan, tab background, atau perangkat terkunci. Kondisi tersebut tidak membuktikan bahwa participant berniat meninggalkan meja. Automatic durable removal setelah 60 detik dapat menghilangkan seat di tengah board dan membuat marker recovery browser tidak lagi cocok dengan membership server.

Browser juga menyimpan identity dan table marker secara terpisah. Marker lokal hanya petunjuk untuk mencoba recovery; marker tersebut tidak membuktikan participant atau seat masih aktif.

## Decision

- WebSocket disconnect hanya mengubah reconstructable presence menjadi offline. Disconnect tidak mengirim domain command, tidak mengubah table revision, dan tidak menghapus participant atau seat.
- Participant dan seat tetap durabel sampai explicit leave, owner removal, table completion cleanup, abandonment policy, atau explicit expiry policy terpisah dijalankan.
- Bootstrap client memakai state `NO_SESSION`, `SESSION_ONLY`, `TABLE_RECOVERING`, `TABLE_ACTIVE`, dan `TABLE_EXPIRED`.
- Identity yang valid dan table marker memicu authenticated `GET /v1/tables/{id}`. Hanya respons projection participant aktif yang menghasilkan `TABLE_ACTIVE`.
- `TABLE_ACTIVE` dari landing atau lobby diarahkan dengan replace navigation ke `/table/{id}`. Halaman table membuka WebSocket lalu subscribe/resume dari revision dan sequence authoritative terbaru.
- Respons table not found/access denied terhadap marker recovery menghasilkan `TABLE_EXPIRED`, menghapus marker lokal, dan kembali ke lobby. Client tidak memanggil join, take-seat, atau mutation lain selama recovery.
- Recovery bergantung pada konsep identity, participant, table, dan seat. Implementasi guest saat ini tidak boleh membuat recovery table bergantung pada cabang UI khusus guest agar user session dapat ditambahkan kemudian tanpa mengubah engine atau table membership.

## Consequences

Positive:

- Reload, sleep, dan gangguan jaringan tidak menghilangkan seat atau merusak board aktif.
- Browser state tidak dipercaya melebihi authoritative participant projection server.
- Reopen dapat kembali ke route table dan resume realtime tanpa mutation tersembunyi.

Negative/trade-offs:

- Meja dapat mempertahankan seat offline sampai owner atau lifecycle cleanup mengambil tindakan eksplisit.
- Abandonment dan retention membutuhkan policy terpisah dari presence transport.
- Invite tidak dapat menggantikan participant offline selama seat masih dimiliki secara durabel.

## Validation

- Realtime tests membuktikan last-connection disconnect menghasilkan offline presence tanpa mengubah participant atau seat.
- Browser E2E menutup context, membuka app dengan identity/table marker yang sama, kembali otomatis ke route table, menerima resume snapshot, dan mempertahankan seat serta board revision.
- Browser E2E dengan marker stale kembali ke lobby dan menghapus marker tanpa join atau seat claim.

## Roadmap boundary

- Tidak menambah account system, remember-me multi-device, guest-to-user migration, atau redesign landing/lobby.
- Table abandonment dan cleanup jangka panjang harus menjadi command/policy eksplisit serta tidak boleh diturunkan langsung dari satu WebSocket disconnect.
