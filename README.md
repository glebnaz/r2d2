# R2D2

Персональный Telegram-бот на Go. Работает на Raspberry Pi.

## Фичи

- **Obsidian Task Reminders** — сканирует Obsidian vault и отправляет напоминания о задачах
  - Утренний дайджест в 9:00 с задачами на день и просроченными
  - Точечные напоминания в указанное время (`due: 2026-03-31T16:00`)
  - Приоритеты, проекты, описания
- **Git Sync** — периодически копирует vault в git-репозиторий и пушит на GitHub
  - Автоматический rsync + commit + push по таймеру
  - Уведомления в Telegram об успешном push и конфликтах
  - Настраиваемый интервал, branch, автор коммитов
- **Prometheus метрики** — бизнес и hardware метрики на `:9182/metrics`
- **Grafana дашборд** — визуализация задач, уведомлений, памяти, CPU

## Быстрый старт

```bash
# Собрать
go build -o r2d2 ./cmd/r2d2

# Запустить (dry-run — вывод в консоль)
./r2d2 --config config.json --dry-run

# Запустить
./r2d2 --config config.json
```

## Конфиг

```json
{
  "vault_path": "/path/to/vault",
  "telegram_token": "BOT_TOKEN",
  "telegram_chat_id": 123456789,
  "timezone": "Europe/Berlin",
  "morning_hour": 9,
  "scan_interval_minutes": 5,
  "reminder_statuses": ["todo", "in-progress", "block"],
  "git_sync": {
    "enabled": true,
    "repo_url": "https://github.com/user/vault.git",
    "work_dir": "/data/git-vault",
    "branch": "main",
    "push_interval_min": 30,
    "author_name": "R2D2 Bot",
    "author_email": "r2d2@bot.local",
    "notify_on_push": true,
    "notify_on_conflict": true
  }
}
```

## Docker

```bash
docker pull ghcr.io/glebnaz/r2d2:master
docker run -v /path/to/config.json:/etc/r2d2/config.json:ro \
           -v /path/to/vault:/vault:ro \
           -v r2d2-git-vault:/data/git-vault \
           ghcr.io/glebnaz/r2d2:master \
           --config /etc/r2d2/config.json
```

## Архитектура

Расширяемая система коллекторов для утреннего дайджеста — новые секции добавляются реализацией интерфейса `digest.Collector`.

Подробнее: [docs/architecture/](docs/architecture/)

- [Планирование напоминаний](docs/architecture/scheduling.md)
- [Digest Engine](docs/architecture/digest-engine.md)
- [Сканирование Vault](docs/architecture/vault-scanning.md)
- [Git Sync](docs/architecture/git-sync.md)
- [Деплой](docs/architecture/deployment.md)

## Тесты

```bash
go test ./...
```
