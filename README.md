# Transjakarta Fleet Management

Backend untuk sistem manajemen armada yang menerima lokasi kendaraan melalui
MQTT, menyimpan riwayat ke PostgreSQL, dan menghasilkan event geofence melalui
RabbitMQ.

## Status

Tahap 1: fondasi proyek dan HTTP health endpoint.

## Requirements

- Go 1.25 atau lebih baru

## Menjalankan Server

```bash
go run ./cmd/server