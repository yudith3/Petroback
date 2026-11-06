# Konfigurasi PetroRun

Pendaftaran PetroRun aktif hanya kalau variabel utama di bawah terisi. Kalau
semuanya kosong, server tetap jalan dan endpoint PetroRun membalas
`PETRORUN_UNAVAILABLE`. Kalau hanya sebagian yang terisi, server menolak start
supaya konfigurasi setengah jadi tidak lolos ke produksi.

```dotenv
PETRORUN_BASE_PRICE_IDR=150000          # harga normal per peserta, rupiah
PETRORUN_ADMIN_FEE_IDR=<biaya admin final per order, boleh 0>
PETRORUN_JERSEY_SURCHARGE_2XL_IDR=<tambahan final, boleh 0>
PETRORUN_JERSEY_SURCHARGE_3XL_IDR=<tambahan final, boleh 0>
PETRORUN_JERSEY_SURCHARGE_4XL_IDR=<tambahan final, boleh 0>
PETRORUN_BANK_NAME=<bank final>
PETRORUN_BANK_ACCOUNT_NUMBER=<nomor rekening final; pertahankan nol awal>
PETRORUN_BANK_ACCOUNT_HOLDER=<atas nama rekening final>
PETRORUN_ACCESS_SECRET=<random, minimal 32 karakter>
```

`PETRORUN_BASE_PRICE_IDR`, `PETRORUN_ADMIN_FEE_IDR`, dan semua harga tambahan
jersey untuk ukuran 2XL, 3XL, dan 4XL dibaca dari environment. Ukuran S sampai
XL tidak memiliki biaya tambahan. Variabel surcharge jersey boleh dikosongkan
atau tidak disetel; nilainya dianggap `0`. Nilai negatif dan angka yang tidak
valid ditolak saat server mulai. Harga dan rekening disalin ke setiap order
saat dibuat, jadi perubahan hanya berlaku untuk order baru.

Order ID mengikuti format `PR27-000123` dengan nomor urut global. Nomor tiket
mengikuti format `PR27-TKT-001` dan juga berurutan global agar setiap tiket
tetap unik di seluruh order.

## Jadwal dan harga bertahap

Keputusan panitia 19 Oktober 2026 menggantikan asumsi satu harga sebelumnya:

- Pendaftaran 24 Oktober 2026 pukul 00:00 WIB sampai **22 November 2026 selesai**.
  Batas API eksklusif adalah 23 November pukul 00:00 WIB.
- Early bird Rp125.000 per peserta untuk kuota 25 peserta. Slot direservasi
  ketika order dibuat selama 24 jam; order menunggu bayar yang kedaluwarsa
  melepaskan slot. Kuota dihitung per peserta, bukan per order.
- Harga normal Rp150.000 per peserta. Grup yang melewati sisa kuota mendapat
  campuran early bird dan normal sesuai urutan peserta dalam request.
- Late registration Rp139.000 per peserta mulai 19 November pukul 00:00 WIB
  (H-3 dari tanggal tutup 22 November), mengesampingkan sisa early bird.
- Pengambilan racepack 25 November 2026. Jam/lokasi belum diberikan.
- Tiket mencakup jersey, medal dan goodie bag. Biaya admin, tambahan ukuran,
  bank/rekening belum diputuskan; jangan memakai nominal/rekening contoh.

Default harga early/late dan jadwal dapat diganti lewat environment berikut.
Kosong berarti memakai default ini, bukan mematikan pemeriksaan tanggal:

```dotenv
PETRORUN_EARLY_BIRD_PRICE_IDR=125000
PETRORUN_EARLY_BIRD_QUOTA=25
PETRORUN_LATE_PRICE_IDR=139000
PETRORUN_REGISTRATION_START_AT=2026-10-24T00:00:00+07:00
PETRORUN_REGISTRATION_END_AT=2026-11-23T00:00:00+07:00
PETRORUN_LATE_START_AT=2026-11-19T00:00:00+07:00
PETRORUN_RACEPACK_AT=2026-11-25T00:00:00+07:00
```

Selama belum siap aktivasi, biarkan **semua** `PETRORUN_*` kosong di `.env`.
Compose staging dan produksi meneruskan semua key ke container. Isi konfigurasi
lengkap pada `.env` milik environment yang dituju lalu deploy backend ulang;
workflow mempertahankan file `.env` server. Merge staging tidak mengaktifkan
produksi maupun memasukkan informasi rekening yang belum diberikan.

`GET /api/v1/petrorun/config` menambahkan `sales` (jadwal/harga/kuota) dan
`availability` (`registration_status`: upcoming/open/closed, `phase`:
early_bird/normal/late, `early_bird_remaining`). Response `no-store`; kuota adalah
estimasi saat dibaca. Harga final dialokasikan di transaksi pembuatan order,
menggunakan lock yang sama dengan expiry/upload/review. Satu order memiliki
snapshot harga per peserta, `pricing.lines` berisi tier/quantity/unit_price_idr/
subtotal_idr dan `pricing.base_subtotal` tetap benar untuk grup campuran.
Invoice lama tanpa snapshot tier tetap memakai harga lamanya.

Retry idempotent, resume, upload bukti, review dan tiket tetap tersedia setelah
pendaftaran ditutup; batas tanggal hanya berlaku untuk **order baru**. Deadline
pembayaran tetap created_at + 24 jam, termasuk order menjelang penutupan.

Order pending review yang sudah memegang reservasi mempertahankan kuota selama
review. Penolakan manual melepaskan slot. Bukti terlambat/pengganti tetap dapat
masuk manual review; server mencoba mengambil kembali slot early bird yang
sudah dilepas. Jika kuota sudah dipakai order lain, bukti tetap diterima tetapi
approval mengembalikan 409 `PETRORUN_EARLY_BIRD_EXHAUSTED` tanpa menerbitkan tiket,
tanpa mengubah tagihan. Panitia perlu menyelesaikan pembayaran tersebut terlebih
dahulu. Slot/tiket tidak boleh melewati kuota karena review terlambat.

## `PETRORUN_ACCESS_SECRET`

Peserta tidak punya akun. Setelah order dibuat, response berisi
`access_token`, yaitu HMAC-SHA256 dari ID order dengan secret ini. Token
tersebut dipakai untuk melihat status dan mengunggah bukti transfer tanpa
login. Tokennya tidak disimpan di database.

- Buat dengan generator acak, misalnya `openssl rand -base64 48`.
- Jangan di-commit dan jangan dipakai ulang dari `JWT_SECRET`.
- Mengganti secret membatalkan token semua order yang sudah ada. Lakukan hanya
  kalau secret bocor, lalu kirim ulang tautan akses ke peserta yang ordernya
  masih aktif.

## Bukti pembayaran

Peserta mengunggah bukti asli ke `POST
/api/v1/petrorun/orders/{order_id}/payment-proofs` dengan header
`Authorization: Bearer <access_token>` dan `multipart/form-data`:

- `proof`: satu file JPEG, PNG, atau PDF, maksimum 5 MiB.
- `ocr`: satu objek JSON hasil OCR di browser (maksimum 64 KiB), dengan
  `classification` (`receipt`, `not_receipt`, atau `unknown`), `raw_text`,
  `sender_name`, `recipient_name`, `transaction_status` (`success`, `failed`,
  atau `unknown`), `extracted_amount`, `destination_account`, `destination_bank`,
  `transaction_at` (RFC3339 dengan offset), `timezone` (zona IANA seperti
  `Asia/Jakarta`), `precision`
  (`minute` atau `second`), `confidence` (0 sampai 1), `reference`, dan
  `order_id`.

Server memeriksa ekstensi dan signature file, menghitung SHA-256, dan
membandingkan nominal tepat (termasuk kode unik), rekening/bank, nama penerima
dengan pemilik rekening, Order ID, dan waktu transaksi terhadap snapshot order.
Bukti hanya auto-approved jika OCR menyebut receipt, status transaksi `success`,
confidence minimal 0.90, seluruh data yang diperlukan cocok, dan transaksi
serta unggahan masih dalam jendela pembayaran 24 jam. Nama pengirim dan
penerima serta status transaksi OCR disimpan dan dikembalikan bersama data
bukti. Hasil OCR
dari browser adalah input yang dapat dimanipulasi klien; karena itu hasil
tersebut tidak boleh dianggap sebagai bukti kriptografis dan bukti meragukan
harus ditinjau admin.

Status bukti yang digunakan: `pending_review`, `approved`, `rejected`, dan
`rejected_non_receipt`. Pengiriman ulang file identik untuk order yang sama
idempotent. Hash identik pada order berbeda masuk antrean review. Tiket hanya
diterbitkan sekali setelah persetujuan, dalam transaksi database yang sama
dengan perubahan status.

Admin yang terautentikasi dapat mengambil antrean di
`GET /api/v1/petrorun/admin/payment-proofs?status=pending_review&page=1&page_size=20`,
mengambil file privat melalui
`GET /api/v1/petrorun/admin/payment-proofs/{proof_id}/file`, lalu memutuskan
melalui `POST /api/v1/petrorun/admin/payment-proofs/{proof_id}/review` dengan
JSON `{"decision":"approve"}` atau `{"decision":"reject","note":"..."}`.
Penolakan memerlukan catatan. Semua keputusan dicatat di audit log.

## Resume order dan dashboard

`GET /api/v1/petrorun/orders/{order_id}` memakai Bearer access token yang sama
dengan upload/tiket, mengembalikan snapshot pricing/bank, status, deadline tetap,
dan waktu server. Response tidak memuat kontak atau medical history. Gunakan
token dalam fragment link frontend, lalu header Authorization; jangan query URL.

Seluruh endpoint `/petrorun/admin/*`, termasuk stream bukti, hanya untuk
`superadmin`. Role `division_admin` OPREC ditolak (403). Belum ada role/scope
PetroRun terpisah, sehingga akses tersebut tidak diberikan secara implisit.

- `GET /api/v1/petrorun/admin/orders`: array order dengan kontak registrant,
  jumlah peserta, snapshot tagihan, deadline, latest_proof dan verification_source.
- `GET /api/v1/petrorun/admin/orders/summary`: total_orders, total_participants,
  waiting_payment, pending_review, approved, rejected, expired dan approved_total.
  Nilai uang approved hanya dihitung satu kali per order.
- `GET /api/v1/petrorun/admin/orders/{order_id}`: detail peserta (termasuk medical
  history), semua proof attempt beserta raw OCR/precision/timezone, file_url privat,
  reviewer/review_note, tiket dan audit actor/from_status/to_status/reason/time.

List/summary menerima `search` (order/ticket/nama/email/WhatsApp), `status`,
`created_from`, `created_to`, `transaction_from`, `transaction_to`, `sort`
(`created_at`, `expires_at`, `payable_total`, `status`), `direction` (`asc`/`desc`),
`page` dan `page_size` (maksimum 100). Tanggal `YYYY-MM-DD` memakai Asia/Jakarta;
tanggal akhir inklusif. Timestamp RFC3339 memakai batas akhir eksklusif.
Response memakai envelope dan metadata pagination existing. UUID/parameter invalid
422, resource hilang 404, anonymous 401 dan keputusan stale 409.

Semua data privat memakai Cache-Control no-store. Pending review tetap terbuka
sesudah deadline. Bukti pengganti setelah penolakan diperbolehkan; attempt lama
dipertahankan. Review attempt yang telah diganti ditolak; retry keputusan yang sama
bersifat idempotent. Unggahan terlambat masuk manual review tanpa mengulang timer.

Kebijakan 19 Oktober 2026: nota dengan nominal berbeda/tidak terbaca, confidence
rendah atau metadata kurang tetap boleh dikirim untuk manual review. Hanya nota
yang lolos seluruh aturan server dapat autoapprove. Order ID internal tidak wajib
tercetak pada nota bank; bila terdeteksi, nilainya harus sesuai. Client tidak boleh
mengisi metadata terdeteksi dengan tebakan agar lolos autoapprove.
