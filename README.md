# rookery-node

Relay-нода для [Rookery](https://github.com/proxt/rookery) — WebRTC-туннеля.
Этот репозиторий содержит только сам relay-сервер (`rookeryd`): он завершает
WebRTC-сессии клиентов и релеит TCP/UDP-трафик. Нода не хранит никакого
состояния сама — пользователи, подписки и статистика трафика живут на
**панели**, которая вместе с Windows-клиентом лежит в основном репозитории
[proxt/rookery](https://github.com/proxt/rookery).

Нода бесполезна сама по себе: сначала её нужно зарегистрировать в работающей
панели (админка → Ноды → Добавить) — оттуда берутся `node_id`/`api_key` для
конфига этой ноды.

## Сборка

```
make build          # bin/rookeryd — статический линукс-бинарь, CGO_ENABLED=0
make docker-build    # локальная сборка Docker-образа (для теста, без публикации)
make test
make lint
```

При каждом пуше в `master` GitHub Actions публикует образ в
`ghcr.io/proxt/rookery-node` (см. `.github/workflows/docker.yml`).

## Установка (Docker, рекомендуется)

1. Сначала зарегистрировать эту ноду в админке панели — скопировать выданные
   `node_id` и `api_key`.
2. Поставить Docker, если его ещё нет: `curl -fsSL https://get.docker.com | sudo sh`
3. На VDS:
   ```
   mkdir -p ~/rookery-node && cd ~/rookery-node
   curl -O https://raw.githubusercontent.com/proxt/rookery-node/master/deploy/docker-compose.yml
   cp configs/node.example.yaml node.yaml   # или сразу создать node.yaml
   ```
   Заполнить `panel_addr`, `node_id`, `api_key` в `node.yaml` (полный список
   полей — в `configs/node.example.yaml`).
4. `sudo docker compose up -d`

`network_mode: host` в compose-файле не опционален — ICE-агенту pion нужно
видеть реальный публичный интерфейс VDS, а не внутренний адрес Docker-моста.

Перед нодой стоит поставить Caddy для TLS — см. `deploy/Caddyfile.example`.

### Порты

- **TCP 443** (или 80 для ACME) — на Caddy.
- **UDP `ice_udp_port`** (по умолчанию 51000) — единственный нужный UDP-порт,
  зафиксирован через `SetICEUDPMux`, диапазон эфемерных портов не используется.
- `listen_addr` (по умолчанию `127.0.0.1:8080`) остаётся внутренним, за Caddy.

## Установка без Docker

См. `deploy/systemd/rookery-node.service` и `deploy/Caddyfile.example`.
Собрать через `make build`, скопировать `bin/rookeryd` в `/opt/rookery/rookeryd`,
указать в `ExecStart` флаг `-config` на свой `node.yaml`.

## Почему отдельный репозиторий

У ноды нет зависимости от панели или клиента, кроме проводного протокола
(токены сессий, подписанные панелью, проверяются здесь собственным
`api_key` ноды — см. `internal/signaling`). Отдельный репозиторий значит,
что relay-ноды можно запускать и обновлять независимо от цикла релизов
панели и клиента, а Docker-образ ноды не требует для сборки ничего из
остального проекта.
