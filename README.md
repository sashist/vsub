# vsub

[English](#english) | [Русский](#русский)

---

## English

Terminal UI tool in Go for converting VPN subscriptions into standard `vless://` links.

### Why

Some VPN providers restrict subscriptions to specific mobile clients (checking `User-Agent: Happ/...` and `X-Hwid` device binding) and return full Xray JSON configurations with routing rules and balancers instead of plain `vless://` links. Standard desktop clients (Nekoray, v2rayN, Hiddify, Throne) often cannot import those subscriptions directly or get blocked by HWID checks.

`vsub` fetches the subscription using a deterministic machine-derived `X-Hwid` (falling back to `v2rayNG` for standard base64/text subscriptions), extracts clean `vless://` URIs (`reality`, `xhttp`, `ws`, `tls`), and lets you browse servers, copy one or all links to the clipboard, or export them to a file.

### Install

Linux / macOS:

```sh
curl -fsSL https://raw.githubusercontent.com/sashist/vsub/main/install.sh | sh
```

With Go:

```sh
go install github.com/sashist/vsub@latest
```

### Usage

```sh
vsub
```

Controls:

- `↑`/`↓` or `j`/`k`, `Enter` — navigate and select (number keys `1`–`5` also work in menus)
- `PgUp`/`PgDn` — scroll the server list (`Enter` copies the selected server)
- `Esc` — back, `q` / `Ctrl+C` — quit

Export options:

- Clipboard (`wl-copy`, `xclip`, `xsel`, `pbcopy`) — single server, full `txt` list, or `base64` subscription
- File — `txt`, `base64`, or `json`

### Build from source

```sh
go build -o vsub .
```

---

## Русский

Терминальная утилита на Go для извлечения `vless://` ссылок из VPN-подписок.

### Зачем нужна

Некоторые VPN-сервисы отдают подписку только мобильным клиентам (проверяют заголовки `Happ` и привязку по `X-Hwid`) и возвращают сырой JSON-конфиг Xray с правилами роутинга и балансировщиками вместо обычного списка `vless://` ссылок. Из-за этого такую подписку нельзя напрямую добавить в стандартные десктопные клиенты (Nekoray, v2rayN, Hiddify, Throne).

`vsub` запрашивает подписку с нужными заголовками и стабильным `X-Hwid` машины (при пустом ответе или ошибке переключается на `v2rayNG`), извлекает чистые `vless://` ссылки (`reality`, `xhttp`, `ws`, `tls`) и позволяет через терминальное меню просмотреть серверы, скопировать один или все сразу в буфер обмена либо сохранить в файл.

### Установка

Linux / macOS:

```sh
curl -fsSL https://raw.githubusercontent.com/sashist/vsub/main/install.sh | sh
```

Через Go:

```sh
go install github.com/sashist/vsub@latest
```

### Использование

```sh
vsub
```

Управление:

- `↑`/`↓` или `j`/`k`, `Enter` — выбор пункта (в меню и подменю также работают цифры)
- `PgUp`/`PgDn` — прокрутка списка серверов (`Enter` копирует выбранный сервер в буфер)
- `Esc` — назад, `q` / `Ctrl+C` — выход

Экспорт:

- буфер обмена (`wl-copy`, `xclip`, `xsel`, `pbcopy`) — один сервер, построчный список или `base64`
- файл — `txt`, `base64` или `json`

### Сборка из исходников

```sh
go build -o vsub .
```
