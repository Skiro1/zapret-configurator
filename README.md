# zapret-configurator

Инструмент для автоматической сборки, конвертации и тестирования конфигов [zapret](https://github.com/bol-van/zapret) и zapret2 для обхода DPI. Загружает конфиги из нескольких источников, конвертирует в единый формат и автоматически подбирает лучший конфиг для вашей сети.

---

## Быстрый старт

### Из релиза (рекомендуется)

1. Скачайте архив из раздела [Releases](https://github.com/Skiro1/zapret-configurator/releases/latest)
2. Распакуйте в любую папку
3. Запустите сборку:
```bash
zapret-configurator all --engine both
```
4. Автоподбор лучшего конфига:
```bash
zapret-configurator autopick --engine zapret2 --mode full --top 10
```

Готовые конфиги появятся в `output/final/zapret/` и `output/final/zapret2/`.

### Из исходников

```bash
git clone <repo-url>
cd zapret-configurator
go build -o zapret-configurator.exe .
zapret-configurator all --engine both
```

---

## Установка

### Из релиза (рекомендуется)

1. Перейдите в раздел [Releases](https://github.com/Skiro1/zapret-configurator/releases/latest)
2. Скачайте последний архив (например `zapret-configurator-windows-amd64.zip`)
3. Распакуйте в любую папку
4. Запустите `zapret-configurator.exe` от имени администратора

### Из исходников

Требуется [Go](https://go.dev/dl/) 1.22 или новее.

```bash
git clone <repo-url>
cd zapret-configurator
go build -o zapret-configurator.exe .
```

Бинарник появится в корне проекта. Папки `output/` создаются автоматически при первом запуске.

---

## Команды

| Команда | Описание |
|---------|----------|
| `sync` | Скачивает конфиги и рантайм из GitHub (Flowseal, youtubediscord, zapret-kvn) |
| `convert` | Конвертирует скачанные конфиги в единый формат `.bat` |
| `build-final` | Собирает финальные папки `output/final/zapret/` и `output/final/zapret2/` |
| `autopick` | Тестирует каждый конфиг через прокси, ранжирует и копирует лучшие |
| `all` | Последовательно выполняет `sync` -> `convert` -> `build-final` |

### Флаги

| Флаг | По умолчанию | Описание |
|------|-------------|----------|
| `--output PATH` | `./output` | Директория для скачивания и сборки |
| `--engine` | `both` | `zapret`, `zapret2` или `both` |
| `--mode` | `quick` | Режим тестирования: `quick` (30), `standard` (80), `full` (все) |
| `--target DOMAIN` | — | Дополнительные домены через запятую |
| `--top N` | `5` | Количество лучших конфигов для копирования в autopick |
| `--zapret2-installer-url URL` | — | Кастомный URL для Zapret2 |
| `--github-token TOKEN` | — | GitHub API токен (или переменная `GITHUB_TOKEN`) |

---

## Примеры

```bash
# Скачать и собрать только zapret
zapret-configurator all --engine zapret

# Скачать и собрать только zapret2
zapret-configurator all --engine zapret2

# Только скачать конфиги
zapret-configurator sync --engine both

# Только конвертировать (если sync уже был)
zapret-configurator convert --engine both

# Только собрать финальную папку
zapret-configurator build-final --engine zapret

# Быстрый автопик (30 конфигов)
zapret-configurator autopick --engine zapret2 --mode quick --top 1

# Полный автопик (все конфиги)
zapret-configurator autopick --engine zapret2 --mode full --top 10

# С доп. доменами
zapret-configurator autopick --engine zapret --mode standard --target rutracker.org

# С GitHub токеном
set GITHUB_TOKEN=ghp_xxxx && zapret-configurator all --engine both
```

---

## Как пользоваться

### Полная сборка

```bash
zapret-configurator all --engine both
```

Это скачает все источники, сконвертирует конфиги и соберёт финальные папки. Результат:

```
output/
  _downloaded/          # временные скачанные файлы
  _converted/           # временные сконвертированные bat
  final/
    zapret/             # готовые конфиги + bin + lists + utils
    zapret2/            # готовые конфиги + bin + exe + lists + lua + windivert.filter
```

### Автоподбор

```bash
zapret-configurator autopick --engine zapret2 --mode full --top 10
```

Протестирует каждый конфиг через прокси-сервер (HTTPS, STUN, UDP). Лучшие конфиги копируются в `output/final/autopick/zapret2/` или `output/final/autopick/zapret/` вместе с необходимыми папками (`bin`, `exe`, `lists`, `lua`, `windivert.filter`).

Результат автопика включает скор и время отклика для каждого протокола.

### Запуск сконвертированных конфигов

1. Скопируйте папку `output/final/zapret/` или `output/final/zapret2/` куда угодно
2. Запустите нужный `.bat` файл от имени администратора
3. Конфиг автоматически подхватит `bin/`, `lists/`, `lua/` и т.д. через относительные пути

---

## Архитектура

```
zapret-configurator/
  main.go                         # CLI точка входа, парсинг флагов
  internal/
    config/
      options.go                  # структура Options
    source/
      github.go                   # GitHub API, скачивание релизов
      sync.go                     # оркестрация скачивания
    bat/
      patcher.go                  # патчинг bat файлов (IFACE_FILTER, exe, аргументы)
    convert/
      convert.go                  # конвертация Flowseal/YD в bat
    runtime/
      build_final.go              # сборка финальных папок
      fileops.go                  # файловые операции
      zapret2.go                  # подготовка рантайма zapret2
    lists/
      lists.go                    # копирование списков из lists_source/
      embedded.go                 # встроенные списки (ipset-ru.txt, other.txt)
    probes/
      probes.go                   # HTTPS, STUN, UDP пробы
    autopick/
      autopick.go                 # тестирование и ранжирование конфигов
    report/
      report.go                   # генерация JSON/Markdown отчётов
    catalog/
      catalog.go                  # парсер каталога YD стратегий
  lists_source/                   # реальные файлы списков (40+ файлов)
  output/                         # результат сборки
```

---

## Проблемы и решения

### Monkey64.sys / WinDivert не удаляются

Если `taskkill` или `del` не могут удалить файлы WinDivert:

```bash
# 1. Остановите все процессы winws.exe / winws2.exe
taskkill /IM winws.exe /F
taskkill /IM winws2.exe /F

# 2. Остановите службу WinDivert
sc stop WinDivert
sc delete WinDivert

# 3. Перезагрузите компьютер

# 4. После перезагрузки удалите файлы
del /f /q "C:\Windows\System32\drivers\Monkey64.sys"
del /f /q "C:\Windows\System32\drivers\WinDivert64.sys"
```

### winws.exe не запускается / instantly завершается

Причины:
- Нет прав администратора (запустите `.bat` от имени администратора)
- Устаревшие аргументы в конфиге (обновите проект)
- Отсутствует `WinDivert.dll` или `Monkey64.sys` рядом с `winws.exe`

### Автопик показывает 0 конфигов

Убедитесь, что `build-final` был выполнен:
```bash
zapret-configurator build-final --engine zapret2
```

### GitHub API возвращает 403 (rate limit)

Используйте токен:
```bash
zapret-configurator sync --github-token ghp_xxxx
```

Лимит без токена: 60 запросов/час. С токеном: 5000 запросов/час.

### Конфиг не работает в папке autopick

Убедитесь, что папки `bin/`, `lists/` и другие зависимости скопированы. Автоматически копируются при использовании `autopick`, но если копировали вручную — скопируйте и их.

---

## Удаление проекта

```bash
# 1. Остановите все процессы
taskkill /IM winws.exe /F
taskkill /IM winws2.exe /F
taskkill /IM zapret-configurator.exe /F

# 2. Удалите директорию проекта
rmdir /s /q "D:\SKKVPN\zapret\zapret-configurator"
```

Если `rmdir` не работает из-за заблокированных файлов:
1. Перезагрузите компьютер
2. Запустите `rmdir /s /q "D:\SKKVPN\zapret\zapret-configurator"` сразу после загрузки

---

## Благодарности

Проект использует результаты работ:

- **[bol-van/zapret](https://github.com/bol-van/zapret)** -- оригинальный zapret, основа для DPI-обхода
- **[Flowseal/zapret-discord-youtube](https://github.com/Flowseal/zapret-discord-youtube)** -- готовые конфиги для Discord и YouTube
- **[youtubediscord/zapret](https://github.com/youtubediscord/zapret)** -- альтернативные конфиги и стратегии
- **[youtubediscord/zapret-kvn](https://github.com/youtubediscord/zapret-kvn)** -- рантайм для zapret2 (winws2.exe + DLL)

---

## Лицензия

Проект распространяется под лицензией [MIT](LICENSE).
