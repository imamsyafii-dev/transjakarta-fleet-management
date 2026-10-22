# Transjakarta Fleet Management

Backend untuk sistem manajemen armada yang menerima lokasi kendaraan melalui
MQTT, menyimpan riwayat ke PostgreSQL, dan menghasilkan event geofence melalui
RabbitMQ.

## Status

Tahap 3: fondasi Go, infrastruktur lokal, dan schema database awal.

## Requirements

- Go 1.25 atau lebih baru

## Menjalankan Server

```bash
go run ./cmd/server
```

Server berjalan pada `http://localhost:8080`.

## Health Check

```bash
curl -i http://localhost:8080/health
```

Respons yang diharapkan:

```json
{"status":"ok"}
```

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

Untuk menguji Mosquitto, jalankan subscriber dan publisher pada dua terminal terpisah:

Login menggunakan `RABBITMQ_DEFAULT_USER` dan `RABBITMQ_DEFAULT_PASS` dari file `.env`.

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
