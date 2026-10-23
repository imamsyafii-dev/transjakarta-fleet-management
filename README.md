# Transjakarta Fleet Management

Backend untuk sistem manajemen armada yang menerima lokasi kendaraan melalui
MQTT dan menyimpan riwayat ke PostgreSQL. Event geofence melalui RabbitMQ akan
ditambahkan pada tahap berikutnya.

## Status

Tahap 7: fondasi Go, infrastruktur lokal, MQTT publisher/subscriber, penyimpanan PostgreSQL asynchronous, dan REST API lokasi tervalidasi.

## Requirements

- Go 1.25 atau lebih baru
- Docker Desktop (termasuk Docker Compose)

## Menjalankan Server

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
| PostgreSQL | Menyimpan riwayat lokasi kendaraan | `localhost:5432` |
| Mosquitto | MQTT broker untuk data lokasi IoT | `localhost:1883` |
| RabbitMQ | Message broker untuk event geofence | `localhost:5672` |
| RabbitMQ Management | Dashboard RabbitMQ | `http://localhost:15672` |

Semua port hanya dapat diakses dari laptop lokal.

## Menjalankan Infrastruktur

Salin konfigurasi environment lokal:

```bash
cp .env.example .env
```

Jalankan seluruh layanan:

```bash
docker compose up -d
docker compose ps
```

Hentikan layanan tanpa menghapus data PostgreSQL dan RabbitMQ:

```bash
docker compose down
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

| Kolom | Fungsi |
| --- | --- |
| `id` | Identitas unik internal setiap data lokasi |
| `vehicle_id` | ID kendaraan |
| `latitude` | Latitude, harus antara -90 dan 90 |
| `longitude` | Longitude, harus antara -180 dan 180 |
| `timestamp` | Unix timestamp dalam satuan detik |

Terapkan migration pertama ke PostgreSQL lokal:

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

Jalankan backend server:

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
data yang sudah ada di antrean, lalu menutup koneksi PostgreSQL.

## Verifikasi Penyimpanan PostgreSQL

Setelah server dan publisher berjalan beberapa detik, periksa lokasi yang
tersimpan:

```bash
docker compose exec -T postgres \
  psql -U fleet -d fleet_management \
  -c "SELECT id, vehicle_id, latitude, longitude, timestamp FROM vehicle_locations ORDER BY id DESC LIMIT 10;"
```

Data `vehicle_id` dan koordinat harus sesuai dengan payload mock publisher.
