# Деплой

## Инфраструктура

- **Raspberry Pi** (arm64, Debian Bookworm)
- **Docker Compose** — вместе с другими сервисами в `home-server` репо
- **Obsidian Headless** — синхронизация vault через Obsidian Sync

## Docker образ

Собирается автоматически через GitHub Actions при пуше в `master`.

- Registry: `ghcr.io/glebnaz/r2d2`
- Платформы: `linux/arm64`, `linux/amd64`
- Тег по ветке: `ghcr.io/glebnaz/r2d2:master`

## docker-compose.yaml

```yaml
r2d2:
  image: ghcr.io/glebnaz/r2d2:master
  container_name: r2d2
  dns:
    - 1.1.1.1
    - 8.8.8.8
  volumes:
    - ./config/r2d2/config.json:/etc/r2d2/config.json:ro
    - /home/glebnaz/Documents/my-vault:/vault:ro
  command: ["--config", "/etc/r2d2/config.json"]
  ports:
    - "9182:9182"
  restart: unless-stopped
```

DNS явно указан — без него контейнер не резолвит `api.telegram.org` (pihole на том же хосте перехватывает).

## Obsidian Sync

На Pi установлен `obsidian-headless` (npm пакет). Работает как systemd сервис:

```
/etc/systemd/system/obsidian-sync.service
ExecStart=/usr/bin/ob sync --continuous --path /home/glebnaz/Documents/my-vault
```

Синхронизирует vault непрерывно. Бот читает vault read-only.

## Мониторинг

- Prometheus скрейпит `r2d2:9182/metrics` каждые 30 секунд
- Grafana dashboard: "R2D2 Bot"

## Обновление

```bash
cd ~/infra/home-server
git pull
docker compose pull r2d2
docker compose up -d r2d2
```
