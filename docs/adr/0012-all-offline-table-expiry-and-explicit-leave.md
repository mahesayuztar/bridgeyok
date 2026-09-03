# ADR-012: All-Offline Table Expiry and Explicit Leave

- Status: Accepted
- Date: 4 September 2026
- Decision owners: Product/Engineering
- Supersedes: retention timing pada OD-03 dan batas explicit leave pada ADR-006/ADR-011

## Context

Guest session berlaku lama agar reload dan gangguan jaringan tidak menghilangkan identitas. Namun meja yang seluruh participant-nya sudah offline dapat tertinggal sebagai resource aktif tanpa batas praktis. Marker recovery di `localStorage` juga membuat link biasa ke lobby tidak cukup sebagai tindakan keluar: tab lain atau reload dapat memulihkan meja tersebut kembali.

Disconnect tunggal tetap tidak boleh langsung menghapus seat. Policy baru perlu membedakan gangguan singkat dari meja yang benar-benar ditinggalkan, mempertahankan revision fence, dan menghapus kemampuan seluruh participant aktif untuk memulihkan meja yang sudah ditutup.

## Decision

- Meja open ditutup setelah dua kondisi berlangsung setidaknya lima menit: tidak ada subscribed connection participant dan tidak ada durable table action.
- `TABLE_INACTIVITY_TIMEOUT` memiliki default `5m`; scan bounded dijalankan setiap `15s` melalui `TABLE_LIFECYCLE_SWEEP_INTERVAL`.
- Waktu aktivitas durable tetap berasal dari `tables.meaningful_at`. Heartbeat, subscribe, resume, dan presence frame tidak memperpanjangnya.
- Waktu all-offline berasal dari broker presence single-instance. Setelah process restart, meja yang belum dikenal dianggap mulai offline saat process hidup agar tidak langsung ditutup ketika client sedang reconnect.
- Candidate expiry membawa revision hasil scan dan dikirim sebagai internal command melalui table actor. Aktivitas yang menang race mengubah revision sehingga expiry ditolak sebagai stale.
- Commit expiry mengubah meja menjadi terminal, menyimpan event `TABLE_EXPIRED`, dan menandai guest session seluruh participant yang masih aktif sebagai `EXPIRED` dalam transaksi PostgreSQL yang sama.
- Realtime menutup connection lain yang menggunakan session participant tersebut dengan `SESSION_INACTIVE`. Client menghapus identity, access token, dan table marker.
- Explicit leave berlaku pada waiting maupun active table. Non-owner melepaskan participant dan seat. Owner memindahkan ownership ke participant aktif lain; bila tidak ada pengganti, meja ditutup dan owner dilepas.
- Tindakan keluar selalu memanggil endpoint leave sebelum navigasi. Keberhasilan menghapus marker table dari `localStorage`; tab lain menangani storage event dengan membersihkan projection dan menghentikan reconnect.
- Link navigasi lobby di gameplay bukan tindakan keluar dan dihapus. Gameplay menyediakan action `Keluar` yang eksplisit.

## Consequences

Positive:

- Meja guest yang benar-benar ditinggalkan berhenti menjadi resource aktif dalam sekitar lima menit.
- Credential participant meja yang expired tidak dapat dipakai untuk refresh, ticket baru, atau recovery.
- Keluar dari satu tab menghentikan recovery dan redirect meja pada tab lain di browser yang sama.
- Reload setelah explicit leave tetap berada di landing/lobby kecuali user sengaja membuka URL meja lagi, yang tetap ditolak oleh membership authoritative.

Negative/trade-offs:

- Participant aktif pada meja yang timeout harus membuat guest identity baru, sesuai permintaan bahwa semua client session meja tersebut expired.
- Scan memiliki toleransi hingga satu sweep interval setelah lima menit.
- Presence tetap process-local sesuai target single-instance. Restart memulai ulang pengukuran all-offline sehingga cleanup dapat tertunda maksimal lima menit tambahan.

## Validation

- Unit test domain untuk leave participant, transfer owner, sole-owner close, dan internal table expiry.
- Unit test lifecycle untuk online guard, five-minute offline guard, revision-fenced expiry command, dan callback hasil commit.
- PostgreSQL integration test membuktikan terminal table state dan invalidasi seluruh active participant session berada dalam satu commit.
- Realtime test membuktikan offline anchor memakai last subscribed connection dan restart grace.
- Web typecheck/test serta browser flow membuktikan leave hanya navigasi setelah server success dan penghapusan marker dipropagasi lintas-tab.

## Roadmap boundary

- Tidak menambah Redis, distributed presence, atau multi-instance lease.
- Session yang sudah explicit leave sebelum table expiry tidak ikut di-expire karena bukan lagi participant aktif.
