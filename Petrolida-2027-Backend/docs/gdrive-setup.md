# Penyimpanan Portofolio di Google Drive

Portofolio pendaftar bisa disimpan di disk server atau di Google Drive milik
panitia. Pilihannya satu baris di `.env`:

```dotenv
PORTFOLIO_STORAGE_DRIVER=local    # disk server, default
PORTFOLIO_STORAGE_DRIVER=gdrive   # Google Drive
```

Pakai `local` saat development supaya Drive panitia tidak terisi berkas
percobaan, dan `gdrive` di produksi.

## Cara kerjanya

Pendaftar tidak pernah berhubungan dengan Google. Mereka mengunggah PDF ke
backend seperti biasa, lalu backend yang meneruskannya ke Drive memakai
kredensial panitia.

```
Pendaftar (form web)
      |  multipart upload
      v
Backend  --- refresh token akun panitia --->  Folder Drive
      |
      +-- menyimpan file ID ke oprec_staff_documents.url
```

Konsekuensinya:

- Pendaftar tidak perlu akun Google dan tidak melihat halaman izin apa pun
- Pendaftar tidak bisa melihat atau menghapus berkas pendaftar lain
- Semua berkas dimiliki akun panitia, jadi tidak bisa dicabut setelah deadline
- Admin membuka portofolio lewat endpoint backend, bukan link Drive publik,
  sehingga foldernya bisa tetap privat

Validasi yang sudah ada tetap berlaku: ekstensi `.pdf`, magic byte `%PDF-`, dan
batas `OPREC_PORTFOLIO_MAX_BYTES`.

## Setup di Google Cloud Console

Semua langkah memakai satu akun Google milik panitia, bukan akun pribadi. Akun
itu nanti jadi pemilik seluruh berkas pendaftar, jadi pilih akun yang bisa
diwariskan ke kepengurusan berikutnya.

1. Buat project di `console.cloud.google.com`
2. **APIs & Services > Library**, cari Google Drive API, klik **Enable**
3. **Google Auth Platform > Branding**, isi nama aplikasi dan email dukungan
4. **Audience**, pilih **External**, lalu tambahkan akun panitia sebagai
   **Test user**
5. **Data Access > Add or remove scopes**, centang hanya:
   ```
   https://www.googleapis.com/auth/drive.file
   ```
6. **Clients > Create client**, Application type **Desktop app**. Salin Client ID
   dan Client secret

### Kenapa hanya `drive.file`

Scope ini membatasi aplikasi pada berkas yang dibuatnya sendiri. Kalau kredensial
bocor, yang bisa disentuh hanya folder portofolio, bukan seluruh isi Drive
panitia.

Scope `drive` penuh termasuk kategori restricted. Untuk mempublikasikan aplikasi
dengan scope itu Google mewajibkan audit keamanan tahunan dari pihak ketiga yang
berbayar, dan selama tidak dipublikasikan refresh token mati setiap 7 hari.

Konsekuensi yang perlu diingat: **folder penampung wajib dibuat oleh aplikasi**.
Folder yang dibuat manual lewat tampilan Drive tidak dikenali, dan unggahan ke
sana akan dijawab `404 File not found`. Perintah di bawah ini yang membuatnya.

## Menerbitkan refresh token

Isi dua nilai dari langkah 6 lebih dulu:

```dotenv
GDRIVE_CLIENT_ID=<client-id>.apps.googleusercontent.com
GDRIVE_CLIENT_SECRET=GOCSPX-<client-secret>
```

Lalu jalankan:

```bash
go run . --gdrive-token
```

Perintah itu membuka halaman izin Google di browser, menunggu persetujuan,
membuat folder penampung, lalu mencetak nilai yang perlu ditempel ke `.env`.

Waktu halaman izin terbuka:

1. Login sebagai akun panitia, bukan akun lain
2. Kalau muncul "Google hasn't verified this app", pilih **Advanced** lalu
   lanjutkan. Wajar untuk aplikasi internal yang tidak mendaftar verifikasi
3. Setujui izin Drive yang diminta

Hasilnya tiga baris:

```dotenv
PORTFOLIO_STORAGE_DRIVER=gdrive
GDRIVE_REFRESH_TOKEN=1//<token panjang>
GDRIVE_FOLDER_ID=<id folder>
```

Kalau `GDRIVE_FOLDER_ID` sudah terisi, perintah ini memakai folder itu dan tidak
membuat yang baru. Nama folder default bisa diganti lewat `GDRIVE_FOLDER_NAME`.

## Wajib: ubah status ke In production

Selama aplikasi berstatus **Testing** di Google Auth Platform, refresh token mati
dalam 7 hari. Kalau ini terlewat, pendaftaran berjalan normal seminggu lalu semua
unggahan gagal mendadak, biasanya tepat saat oprec sedang ramai.

Buka **Audience > Publish app**. Google mensyaratkan halaman Branding terisi
lengkap lebih dulu, termasuk **Application home page** dan **Privacy policy URL**
pada domain yang sudah diverifikasi di Google Search Console.

Terbitkan ulang refresh token setelah publish. Token yang diterbitkan saat status
masih Testing sudah membawa umur 7 hari, dan mengubah status aplikasi tidak
memperpanjangnya.

## Daftar environment variable

| Variabel | Wajib | Keterangan |
|---|---|---|
| `PORTFOLIO_STORAGE_DRIVER` | tidak | `local` (default) atau `gdrive` |
| `PORTFOLIO_STORAGE_DIR` | tidak | Folder disk untuk driver `local`, default `./data/portfolios` |
| `GDRIVE_CLIENT_ID` | ya untuk `gdrive` | Dari menu Clients |
| `GDRIVE_CLIENT_SECRET` | ya untuk `gdrive` | Dari menu Clients, rahasia |
| `GDRIVE_REFRESH_TOKEN` | ya untuk `gdrive` | Dari `--gdrive-token`, rahasia |
| `GDRIVE_FOLDER_ID` | ya untuk `gdrive` | Dari `--gdrive-token` |
| `GDRIVE_TIMEOUT` | tidak | Batas waktu satu operasi Drive, default `1m` |
| `GDRIVE_FOLDER_NAME` | tidak | Nama folder saat dibuat pertama kali |

Server menolak menyala kalau driver `gdrive` dipilih tapi ada nilai yang kosong,
supaya kesalahan konfigurasi ketahuan saat deploy dan bukan saat pendaftar
pertama menekan tombol kirim.

`GDRIVE_CLIENT_SECRET` dan `GDRIVE_REFRESH_TOKEN` adalah kredensial penuh.
Pemegang keduanya bisa membaca dan menghapus semua portofolio. Simpan sebagai
environment variable di server, jangan masuk repo dan jangan dikirim lewat chat.

## Kuota

Akun Google gratis punya 15GB yang dipakai bersama Gmail, Drive, dan Photos.
Dengan batas 10MB per portofolio, 300 pendaftar berarti maksimal sekitar 3GB.
Kalau pendaftarnya jauh lebih banyak, turunkan `OPREC_PORTFOLIO_MAX_BYTES`.

## Masalah yang sering muncul

| Pesan | Artinya |
|---|---|
| `invalid_grant` saat server jalan | Refresh token dicabut, atau aplikasi masih berstatus Testing dan tokennya lewat 7 hari |
| `404 File not found` saat unggah | `GDRIVE_FOLDER_ID` menunjuk folder yang tidak dibuat aplikasi ini |
| `storageQuotaExceeded` | Kredensial yang dipakai milik service account, bukan akun biasa. Service account tidak punya kuota Drive |
| `access_denied` saat `--gdrive-token` | Akun yang login belum terdaftar sebagai Test user di menu Audience |
| `redirect_uri_mismatch` | OAuth client-nya bukan tipe Desktop app |

## Pindah dari local ke gdrive

Berkas lama di disk tidak ikut pindah. Setelah driver diganti, portofolio baru
masuk ke Drive sedangkan yang lama tetap di disk dan tidak lagi terbaca, karena
key di kolom `url` bentuknya berbeda. Ganti driver sebelum pendaftaran dibuka,
bukan di tengah jalan.
