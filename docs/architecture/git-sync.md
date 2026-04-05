# Git Sync

## Назначение

Периодически копирует содержимое Obsidian vault в отдельный git-репозиторий и пушит на GitHub. Vault примонтирован read-only — бот не модифицирует его напрямую.

## Поток данных

```
Obsidian Vault (read-only mount)
        │
        │  rsync -a --delete --exclude .git --exclude .obsidian
        ▼
Git Working Tree (work_dir volume)
        │
        │  git add -A → git diff --cached → git commit → git push
        ▼
GitHub Repository
        │
        │  (on success)  → Telegram: 📤 push notification
        │  (on conflict) → Telegram: 🚨 conflict alert
        ▼
Telegram Bot API
```

## Компоненты

- **`internal/gitsync/gitsync.go`** — `Syncer` struct с методами `Run`, `sync`, `ensureRepo`, `git`
- **`internal/gitsync/format.go`** — форматирование уведомлений на русском

## Жизненный цикл

1. `main.go` проверяет `cfg.GitSync.Enabled` и создаёт `gitsync.Syncer`
2. `Syncer.Run(ctx)` запускается в отдельной горутине
3. `ensureRepo` — клонирует репозиторий если work_dir пуст, иначе проверяет remote URL и делает fetch
4. Запускается ticker с интервалом `push_interval_min` минут
5. На каждый тик:
   - rsync vault → work_dir (исключая `.git` и `.obsidian`)
   - `git add -A` — стейджит все изменения
   - `git diff --cached --quiet` — проверяет есть ли изменения
   - Если изменений нет — пропускает
   - Коммит с сообщением `vault sync: N files changed` + diff stat
   - Push в origin/branch
   - При конфликте (non-fast-forward/rejected) — отправляет alert в Telegram
   - При успехе — отправляет notification в Telegram (если включено)
6. Останавливается при отмене контекста (SIGINT/SIGTERM)

## Конфигурация

Все поля находятся в `git_sync` секции конфига:

| Поле | Обязательное | По умолчанию | Описание |
|------|-------------|--------------|----------|
| `enabled` | да | `false` | Включить git sync |
| `repo_url` | да (если enabled) | — | URL репозитория |
| `work_dir` | да (если enabled) | — | Путь к git working tree |
| `branch` | нет | `main` | Ветка для push |
| `push_interval_min` | нет | `30` | Интервал синхронизации (мин) |
| `author_name` | нет | `R2D2 Bot` | Имя автора коммитов |
| `author_email` | нет | `r2d2@bot.local` | Email автора коммитов |
| `notify_on_push` | нет | `true` | Уведомлять об успешном push |
| `notify_on_conflict` | нет | `true` | Уведомлять о конфликтах |

## Метрики

- `r2d2_git_syncs_total` — общее число циклов синхронизации
- `r2d2_git_pushes_total` — успешные push
- `r2d2_git_push_errors_total` — ошибки push
- `r2d2_git_conflicts_total` — обнаруженные конфликты
- `r2d2_git_sync_duration_seconds` — длительность цикла (histogram)
- `r2d2_git_files_changed_last` — количество изменённых файлов в последнем цикле (gauge)

## Решения

- **rsync вместо file watcher**: Vault примонтирован read-only, fsnotify не нужен. rsync с `--delete` гарантирует полную синхронизацию за один проход.
- **Отдельный working tree**: Бот не трогает vault. Git-операции происходят в отдельной директории (Docker volume).
- **Конфликты не разрешаются автоматически**: При non-fast-forward push бот отправляет alert и ждёт ручного разрешения. Это безопаснее, чем автоматический force-push или rebase.
- **Notifier interface**: Тот же интерфейс `SendMessage` что и у scheduler — позволяет использовать dry-run sender для тестов.
