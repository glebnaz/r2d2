# Digest Engine

## Концепция

Утренний дайджест собирается из секций. Каждая секция — это коллектор, который умеет собирать данные и форматировать их.

## Интерфейс

```go
type Collector interface {
    Collect(ctx context.Context, now time.Time) (*Section, error)
}

type Section struct {
    Header string  // например "📝 *Задачи*"
    Body   string  // отформатированный текст
}
```

- Возвращает `nil` — секция пропускается
- Возвращает `Section` — добавляется в дайджест

## Движок

```go
engine := digest.NewEngine()
engine.Register(collector1)
engine.Register(collector2)

msg, err := engine.Build(ctx, time.Now())
```

`Build()` вызывает все коллекторы по порядку регистрации и собирает сообщение:

```
☀️ Доброе утро!
31 March 2026

📝 Задачи
... (секция от TasksCollector)

🌤 Погода
... (секция от гипотетического WeatherCollector)
```

Если ни один коллектор не вернул данных — возвращает пустую строку, дайджест не отправляется.

## Как добавить коллектор

1. Создать файл в `internal/digest/`, например `weather_collector.go`
2. Реализовать `Collector` интерфейс
3. Зарегистрировать в `cmd/r2d2/main.go`:
   ```go
   engine.Register(digest.NewWeatherCollector(...))
   ```
