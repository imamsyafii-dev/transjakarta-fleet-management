# Transjakarta Fleet Management

Backend untuk sistem manajemen armada yang menerima lokasi kendaraan melalui
MQTT, menyimpan riwayat ke PostgreSQL, dan mengirim event masuk geofence ke
RabbitMQ.

## Status

Tahap 9: seluruh sistem dapat dijalankan sebagai container Docker Compose.

## Requirements

- Go 1.25 atau lebih baru
- Docker Desktop (termasuk Docker Compose)

## Menjalankan Server Secara Lokal

Untuk demo utama, gunakan Docker Compose di bagian [Menjalankan Seluruh
Sistem](#menjalankan-seluruh-sistem). Bagian ini berguna bila proses Go ingin
dijalankan langsung dari laptop sementara layanan pendukung tetap memakai Docker.

```bash
unset GOROOT GOPATH

set -a
source .env
set +a

go run ./cmd/server
```

Aplikasi Go tidak memuat file `.env` secara otomatis. Karena itu environment perlu
dimasukkan ke shell dengan `source .env` sebelum server dijalankan.

Server berjalan pada `http://localhost:8080`.

## Health Check

```bash
curl -i http://localhost:8080/health
```

Respons yang diharapkan:

```json
{"status":"ok"}
```

## REST API Lokasi

### Lokasi Terakhir Kendaraan

```text
GET /vehicles/{vehicle_id}/location
```

Contoh:

```bash
curl -i http://localhost:8080/vehicles/B1234XYZ/location
```

Respons sukses (`200 OK`):

```json
{
  "vehicle_id": "B1234XYZ",
  "latitude": -6.2088,
  "longitude": 106.8456,
  "timestamp": 1715003456
}
```

Jika kendaraan belum memiliki lokasi, API mengembalikan `404`:

```json
{"error":"vehicle location not found"}
```

### Riwayat Lokasi Kendaraan

```text
GET /vehicles/{vehicle_id}/history?start={unix_timestamp}&end={unix_timestamp}
```

Parameter `start` dan `end` wajib berupa Unix timestamp positif. Rentang waktu
bersifat inklusif dan hasil diurutkan dari lokasi paling lama ke terbaru.

Contoh:

```bash
curl -i "http://localhost:8080/vehicles/B1234XYZ/history?start=1&end=2000000000"
```

Respons sukses (`200 OK`) adalah JSON array. Jika tidak ada data pada rentang
tersebut, responsnya tetap `200 OK` dengan array kosong:

```json
[]
```

Request dengan `vehicle_id`, `start`, atau `end` yang tidak valid mengembalikan
`400 Bad Request`.

## Postman Collection

Import file berikut ke Postman untuk menguji health check dan kedua endpoint API:

```text
postman/transjakarta-fleet-management.postman_collection.json
```

Collection menyediakan variable `base_url`, `vehicle_id`, `start`, dan `end`.

## Infrastruktur Lokal

Proyek memakai Docker Compose untuk menjalankan layanan pendukung berikut.

| Service | Fungsi | Akses dari laptop |
| --- | --- | --- |
| Server | HTTP API, MQTT subscriber, penyimpanan lokasi, dan RabbitMQ publisher | `http://localhost:8080` |
| Geofence worker | Membaca dan memproses event `geofence_alerts` | — |
| PostgreSQL | Menyimpan riwayat lokasi kendaraan | `localhost:5432` |
| Mosquitto | MQTT broker untuk data lokasi IoT | `localhost:1883` |
| RabbitMQ | Message broker untuk event geofence | `localhost:5672` |
| RabbitMQ Management | Dashboard RabbitMQ | `http://localhost:15672` |

Semua port hanya dapat diakses dari laptop lokal.

## Menjalankan Seluruh Sistem

Salin konfigurasi environment lokal:

```bash
cp .env.example .env
```

Build image Go dan jalankan seluruh layanan:

```bash
docker compose up --build -d
docker compose ps
```

Pada volume PostgreSQL yang masih kosong, migration pertama dijalankan otomatis
ketika PostgreSQL melakukan inisialisasi. Setelah semua service berstatus
berjalan, publisher mock di laptop dapat langsung mengirim ke Mosquitto melalui
`localhost:1883`.

Lihat log komponen aplikasi:

```bash
docker compose logs -f server geofence-worker
```

Hentikan layanan tanpa menghapus data PostgreSQL dan RabbitMQ:

```bash
docker compose down
```

Untuk menghapus seluruh data lokal dan menginisialisasi schema dari awal,
gunakan perintah berikut dengan hati-hati:

```bash
docker compose down -v
```

## Verifikasi Layanan

Cek PostgreSQL:

```bash
docker compose exec postgres \
  psql -U fleet -d fleet_management \
  -c "SELECT current_database(), current_user;"
```

RabbitMQ Management tersedia di `http://localhost:15672`.

Login menggunakan `RABBITMQ_DEFAULT_USER` dan `RABBITMQ_DEFAULT_PASS` dari file `.env`.

Untuk menguji Mosquitto, jalankan subscriber dan publisher pada dua terminal terpisah:

```bash
docker compose exec mosquitto \
  mosquitto_sub -h localhost -t 'fleet/demo' -C 1 -v
```

```bash
docker compose exec mosquitto \
  mosquitto_pub -h localhost -t 'fleet/demo' -m 'hello from MQTT'
```

## Database Migration

Migration SQL bersifat append-only. Jangan mengubah migration yang sudah pernah
diterapkan; buat file migration baru dengan nomor berikutnya.

Migration pertama membuat tabel `vehicle_locations` untuk menyimpan riwayat
lokasi kendaraan.

> Catatan: spesifikasi mencantumkan nama tabel `vehicle_loctions`. Implementasi
> menggunakan `vehicle_locations` sebagai koreksi penamaan, tanpa mengubah
> struktur kolom atau perilaku yang diminta.

| Kolom | Fungsi |
| --- | --- |
| `id` | Identitas unik internal setiap data lokasi |
| `vehicle_id` | ID kendaraan |
| `latitude` | Latitude, harus antara -90 dan 90 |
| `longitude` | Longitude, harus antara -180 dan 180 |
| `timestamp` | Unix timestamp dalam satuan detik |

Docker Compose memasang migration ini pada inisialisasi **volume PostgreSQL
baru**. Jika menjalankan server Go langsung dari laptop terhadap PostgreSQL yang
belum memiliki tabel, terapkan migration pertama secara manual:

```bash
docker compose exec -T postgres \
  psql -v ON_ERROR_STOP=1 -U fleet -d fleet_management \
  < migrations/000001_create_vehicle_locations.up.sql
```

Verifikasi schema:

```bash
docker compose exec postgres \
  psql -U fleet -d fleet_management \
  -c '\d+ vehicle_locations'
```

Migration `down` hanya untuk reset database lokal dan akan menghapus tabel.
Jangan jalankan pada data yang ingin disimpan.

## Mock MQTT Publisher

Program mock publisher meniru perangkat GPS pada kendaraan. Program ini mengirim
lokasi kendaraan ke Mosquitto setiap dua detik.

Topic MQTT:

```text
/fleet/vehicle/{vehicle_id}/location
```

Jalankan publisher dari root project:

```bash
unset GOROOT GOPATH
go run ./cmd/publisher
```

Konfigurasi dapat diubah melalui environment variable:

| Variable | Default | Fungsi |
| --- | --- | --- |
| `MQTT_BROKER` | `tcp://127.0.0.1:1883` | Alamat MQTT broker |
| `VEHICLE_ID` | `B1234XYZ` | ID kendaraan mock |
| `MQTT_CLIENT_ID` | `fleet-publisher-{vehicle_id}` | ID koneksi MQTT |
| `PUBLISH_INTERVAL` | `2s` | Interval pengiriman lokasi |

Contoh menjalankan kendaraan lain setiap satu detik:

```bash
VEHICLE_ID=B5678ABC PUBLISH_INTERVAL=1s go run ./cmd/publisher
```

Lihat pesan yang dikirim dengan subscriber Mosquitto:

```bash
docker compose exec mosquitto \
  mosquitto_sub -h localhost -t '/fleet/vehicle/+/location' -v
```

Publisher mengirim payload JSON dengan QoS 1 dan `retained=false`.
Tekan `Ctrl+C` untuk menghentikannya secara rapi.

## MQTT Subscriber

Backend server berlangganan ke topic berikut:

```text
/fleet/vehicle/+/location
```

Wildcard `+` membuat server menerima lokasi dari semua kendaraan.

Jalankan backend server secara manual hanya bila service `server` Compose sedang
tidak digunakan; keduanya memakai port HTTP yang sama:

```bash
unset GOROOT GOPATH

set -a
source .env
set +a

go run ./cmd/server
```

Konfigurasi server:

| Variable | Default | Fungsi |
| --- | --- | --- |
| `HTTP_ADDRESS` | `:8080` | Alamat HTTP server |
| `MQTT_BROKER` | `tcp://127.0.0.1:1883` | Alamat MQTT broker |
| `MQTT_CLIENT_ID` | `fleet-server` | ID koneksi MQTT server |
| `DATABASE_URL` | — (wajib) | URL koneksi PostgreSQL |
| `RABBITMQ_URL` | `amqp://fleet:fleet_local_password@127.0.0.1:5672/` | URL koneksi RabbitMQ |
| `GEOFENCE_LATITUDE` | `-6.2090` | Latitude pusat geofence demo |
| `GEOFENCE_LONGITUDE` | `106.8456` | Longitude pusat geofence demo |
| `GEOFENCE_RADIUS_METERS` | `50` | Radius geofence dalam meter |
| `GEOFENCE_EVENT_TIMEOUT` | `3s` | Batas waktu publish event RabbitMQ |

Setiap pesan diperiksa sebelum diproses:

- Topic harus cocok dengan `/fleet/vehicle/{vehicle_id}/location`.
- Payload harus berupa satu objek JSON valid.
- Field asing pada JSON ditolak.
- `vehicle_id` payload harus sama dengan ID pada topic.
- Latitude, longitude, dan timestamp harus valid.

Setiap lokasi valid dicatat di log lalu dimasukkan ke antrean internal berukuran
100 data. Worker terpisah menyimpan data dari antrean tersebut ke tabel
`vehicle_locations` di PostgreSQL. Operasi database memiliki timeout tiga detik,
sehingga callback MQTT tidak tertahan oleh database.

Saat server dihentikan, backend berhenti menerima pesan MQTT baru, menyelesaikan
data yang sudah ada di antrean (termasuk event geofence-nya), lalu menutup
koneksi RabbitMQ dan PostgreSQL.

## Verifikasi Penyimpanan PostgreSQL

Setelah server dan publisher berjalan beberapa detik, periksa lokasi yang
tersimpan:

```bash
docker compose exec -T postgres \
  psql -U fleet -d fleet_management \
  -c "SELECT id, vehicle_id, latitude, longitude, timestamp FROM vehicle_locations ORDER BY id DESC LIMIT 10;"
```

Data `vehicle_id` dan koordinat harus sesuai dengan payload mock publisher.

## Geofence dan RabbitMQ

Setelah lokasi berhasil tersimpan di PostgreSQL, backend menghitung jarak titik
lokasi dari pusat geofence menggunakan rumus Haversine. Event hanya dibuat ketika
kendaraan berubah dari **di luar** menjadi **di dalam** radius. Lokasi pertama
sebuah kendaraan hanya menetapkan status awal, sehingga tidak langsung dianggap
sebagai entry.

Untuk data mock bawaan, pusat `(-6.2090, 106.8456)` dengan radius 50 meter
membuat rute mock bergerak dari luar ke dalam geofence sekali pada setiap siklus.
Backend menerbitkan event berikut ke exchange `fleet.events`, routing key
`geofence.entry`, lalu RabbitMQ meneruskannya ke queue `geofence_alerts`:

```json
{
  "vehicle_id": "B1234XYZ",
  "event": "geofence_entry",
  "location": {
    "latitude": -6.2094,
    "longitude": 106.8456
  },
  "timestamp": 1715003456
}
```

Jalankan worker secara manual di terminal terpisah setelah RabbitMQ aktif. Jika
`geofence-worker` sudah berjalan lewat Docker Compose, jangan menjalankan worker
manual kedua kecuali memang ingin membagi pesan queue:

```bash
unset GOROOT GOPATH

set -a
source .env
set +a

go run ./cmd/geofenceworker
```

Worker memvalidasi, mencatat, lalu melakukan acknowledgement untuk setiap event
dari `geofence_alerts`.

### Demo Docker Compose

Setelah `docker compose up --build -d`, jalankan publisher mock dari laptop:

```bash
unset GOROOT GOPATH
go run ./cmd/publisher
```

Pada terminal lain, lihat event dan penyimpanan lokasi:

```bash
docker compose logs -f server geofence-worker
```

Backend container menerima MQTT dari Mosquitto, menyimpan lokasi ke PostgreSQL,
lalu worker container mencatat event `processed geofence event` ketika rute mock
memasuki geofence.

### Demo Manual Tiga Terminal

1. Terminal A: jalankan `go run ./cmd/server`.
2. Terminal B: jalankan `go run ./cmd/geofenceworker`.
3. Terminal C: jalankan `go run ./cmd/publisher`.

Terminal A akan mencatat lokasi yang diterima dan terminal B akan mencatat
`processed geofence event` saat rute mock memasuki area. Bila ingin melihat
jumlah pesan yang menunggu di queue melalui dashboard, hentikan worker sementara
lalu buka `http://localhost:15672`, masuk ke queue `geofence_alerts`, dan mulai
lagi worker untuk memprosesnya.

Status masuk/keluar geofence disimpan di memori proses. Untuk kebutuhan demo ini,
restart backend akan mengosongkan status tersebut; sistem produksi sebaiknya
menyimpan status geofence secara persisten atau membuat proses event idempoten.
