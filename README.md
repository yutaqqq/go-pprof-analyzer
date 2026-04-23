# go-pprof-analyzer

[![CI](https://github.com/yutaqqq/go-pprof-analyzer/actions/workflows/test.yml/badge.svg)](https://github.com/yutaqqq/go-pprof-analyzer/actions/workflows/test.yml)

CLI-инструмент для автоматического анализа Go pprof-профилей. Находит топ аллокаторов памяти, горячие CPU-пути и утечки горутин. Генерирует отчёт с конкретными рекомендациями.

---

## Реальный кейс: 2.1 GB → 280 MB

При анализе сервиса с нагрузкой 5000 RPS heap-профиль показал:

```
$ pprof-analyzer analyze heap.pb.gz --top 5

## Heap: топ аллокаторов

| Функция                     | Flat    | Flat%  | Рекомендация                                              |
|-----------------------------|---------|--------|-----------------------------------------------------------|
| encoding/json.Marshal       | 1.68 GB | 80.2%  | JSON-аллокации в горячем пути. Используйте прямой маппинг |
| bytes.(*Buffer).WriteString | 180 MB  | 8.6%   | Частые аллокации буфера. Рассмотрите sync.Pool            |
| strings.Builder.grow        | 120 MB  | 5.7%   | Инициализируйте с нужной ёмкостью: make([]T, 0, n)        |
```

**Проблема:** каждый запрос создавал промежуточную `responseDTO` структуру, сериализовал её через `json.Marshal`, а затем отбрасывал. При 5000 RPS это давало 1.8 GB аллокаций в секунду и постоянное давление на GC.

**Решение:** прямой маппинг полей в `json.Encoder` + `sync.Pool` для переиспользования буфера:

```go
var bufPool = sync.Pool{New: func() any { return &bytes.Buffer{} }}

func writeResponse(w io.Writer, data *Data) error {
    buf := bufPool.Get().(*bytes.Buffer)
    buf.Reset()
    defer bufPool.Put(buf)
    return json.NewEncoder(buf).Encode(data)
}
```

**Результат:** RSS: 2.1 GB → 280 MB. Latency p99: 340 ms → 45 ms.

---

## Установка

```bash
go install github.com/yutaqqq/go-pprof-analyzer/cmd/pprof-analyzer@latest
```

---

## Команды

### `analyze` — анализ одного профиля

```bash
# heap-профиль
pprof-analyzer analyze heap.pb.gz --top 20

# CPU-профиль
pprof-analyzer analyze cpu.pb.gz --format json --output report.json

# goroutine-профиль
pprof-analyzer analyze goroutine.pb.gz
```

Тип профиля определяется автоматически по содержимому.

### `diff` — сравнение до/после оптимизации

```bash
pprof-analyzer diff before.pb.gz after.pb.gz --top 10
```

Показывает функции, чьи аллокации выросли, уменьшились, появились или исчезли.

### `leak` — детектор утечек горутин

```bash
# Сделать два снимка с интервалом
curl -s http://localhost:6060/debug/pprof/goroutine > goroutine1.pb
sleep 60
curl -s http://localhost:6060/debug/pprof/goroutine > goroutine2.pb

pprof-analyzer leak goroutine1.pb goroutine2.pb --min-delta 10
```

Находит стеки, чей счётчик горутин вырос на `--min-delta` и более — признак утечки.

---

## Форматы вывода

```bash
# Markdown (по умолчанию) — удобно для PR-комментариев и вики
pprof-analyzer analyze heap.pb.gz -o report.md

# JSON — для интеграции с CI или дашбордами
pprof-analyzer analyze heap.pb.gz -f json -o report.json
```

---

## Флаги

| Флаг           | По умолчанию | Описание                                   |
|----------------|--------------|--------------------------------------------|
| `--top`, `-n`  | `20`         | Число топ-записей в отчёте                 |
| `--output`, `-o` | stdout     | Файл для записи отчёта                     |
| `--format`, `-f` | `markdown` | Формат отчёта: `markdown` или `json`       |
| `--min-delta`, `-d` | `5`    | Минимальный прирост горутин (команда leak) |

---

## Рекомендации движка

Инструмент автоматически генерирует рекомендации на основе паттернов:

| Паттерн                          | Рекомендация                                              |
|----------------------------------|-----------------------------------------------------------|
| `encoding/json` в горячем пути   | Прямой маппинг + easyjson или sonic                       |
| Один аллокатор > 30% от общего   | sync.Pool или arena-аллокатор                             |
| `fmt.Sprintf` в цикле            | strings.Builder или заранее выделенный буфер              |
| `growslice` / `append`           | make([]T, 0, n) с предварительной ёмкостью               |
| Lock/Mutex в горячем пути (CPU)  | Sharded mutex или lock-free структуры                     |
| GC-функции > 5% CPU              | Сократите аллокации: sync.Pool, переиспользование буферов |

---

## Получение профилей

```go
// В коде: включить pprof-эндпоинты
import _ "net/http/pprof"
go http.ListenAndServe(":6060", nil)
```

```bash
# heap (аллокации за всё время)
go tool pprof -proto http://localhost:6060/debug/pprof/heap > heap.pb.gz

# CPU (30 секунд сэмплирования)
go tool pprof -proto http://localhost:6060/debug/pprof/profile?seconds=30 > cpu.pb.gz

# goroutines (текущий снимок)
curl http://localhost:6060/debug/pprof/goroutine?debug=0 > goroutine.pb.gz
```

---

## Лицензия

MIT
