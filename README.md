# hola-proxy

Standalone Hola proxy client. Запускается локально и поднимает HTTP-прокси-сервер, который туннелирует трафик через выбранную страну Hola. По умолчанию слушает `127.0.0.1:8080`.

Это форк [Snawoot/hola-proxy](https://github.com/Snawoot/hola-proxy) с переработанной структурой и обновлёнными зависимостями. Ключевые отличия от оригинала:

- HTTP-клиент на [`enetx/surf`](https://github.com/enetx/surf) с Chrome v145 JA3 fingerprint и порядком заголовков как у настоящего браузера
- TLS-туннель к Hola-агентам с актуальным Chrome ClientHello (`utls.HelloChrome_Auto`) вместо устаревшего `HelloAndroid_11_OkHttp`
- Стандартная Go-структура (`cmd/` + `internal/`), без глобальных мутабельных переменных
- Hola API инкапсулирован в `hola.Client{}` struct с явной конфигурацией

Поддерживается два типа upstream'а: датацентровые прокси (`-proxy-type direct`, по умолчанию) и residental IP пиров в выбраной стране (`-proxy-type lum`). Этот клиент не делает твою машину частью сети Hola — никакие чужие пользователи через тебя не ходят.

---

## Сборка

Требуется Go 1.26+.

### Из исходников

```bash
make native
# бинарь будет в bin/hola-proxy
```

Или через `go install`:

```bash
go install github.com/NeozonS/hola-proxy/cmd/hola-proxy@latest
```

### Под все платформы

```bash
make all       # linux/darwin/windows/freebsd/netbsd/openbsd
make allplus   # + android (требует NDK)
```

### Полезные таргеты

```bash
make fmt   # gofmt
make vet   # go vet
make test  # go test
make tidy  # go mod tidy
make clean # rm -f bin/*
```

---

## Использование

Список доступных стран:

```
$ ./bin/hola-proxy -list-countries
ar - Argentina
at - Austria
au - Australia
be - Belgium
bg - Bulgaria
br - Brazil
ca - Canada
ch - Switzerland
cl - Chile
co - Colombia
cz - Czech Republic
de - Germany
dk - Denmark
es - Spain
fi - Finland
fr - France
gb - United Kingdom (Great Britain)
gr - Greece
hk - Hong Kong
hr - Croatia
hu - Hungary
id - Indonesia
ie - Ireland
il - Israel
in - India
is - Iceland
it - Italy
jp - Japan
kr - Korea, Republic of
mx - Mexico
nl - Netherlands
no - Norway
nz - New Zealand
pl - Poland
pt - Portugal
ro - Romania
ru - Russian Federation
se - Sweden
sg - Singapore
sk - Slovakia
tr - Turkey
uk - United Kingdom
us - United States of America
```

Запустить прокси через выбранную страну:

```
$ ./bin/hola-proxy -country de
```

Через residential IP:

```
$ ./bin/hola-proxy -proxy-type lum -country de
```

Получить список агентов и креды без запуска самого прокси:

```
$ ./bin/hola-proxy -country de -list-proxies -limit 3
Login: user-uuid-a6cf86bec8e342499c9568cc66777735-is_prem-0
Password: 8a30680e98fa
Proxy-Authorization: basic dXNlci11dWlkLWE2Y2Y4N...

host,ip_address,direct,peer,hola,trial,trial_peer,vendor
zagent2799.hola.org,207.154.250.218,22222,22223,22224,22225,22226,digitalocean
zagent2798.hola.org,207.154.248.186,22222,22223,22224,22225,22226,digitalocean
zagent2803.hola.org,157.230.111.167,22222,22223,22224,22225,22226,digitalocean
```

После запуска прокси проверь работу:

```bash
curl -x http://127.0.0.1:8080 https://ifconfig.me
```

---

## Список флагов

| Флаг | Тип | Описание |
| --- | --- | --- |
| `-bind-address` | string | адрес HTTP-прокси (по умолчанию `127.0.0.1:8080`) |
| `-country` | string | код страны (по умолчанию `us`) |
| `-proxy-type` | string | `direct` (датацентр) или `lum` (residential) |
| `-list-countries` | bool | вывести список стран и выйти |
| `-list-proxies` | bool | вывести список агентов и креды |
| `-limit` | uint | количество агентов в `-list-proxies` (по умолчанию 3) |
| `-rotate` | duration | период ротации user UUID (по умолчанию 48h) |
| `-timeout` | duration | таймаут сетевых операций (по умолчанию 35s) |
| `-resolver` | string | DoH/DoT-резолвер (по умолчанию Cloudflare DoH) |
| `-cafile` | string | путь к кастомному CA bundle |
| `-proxy` | string | базовый прокси для всех соединений: `<http\|https\|socks5\|socks5h>://[login:password@]host[:port]` |
| `-user-agent` | string | переопределить User-Agent (по умолчанию — актуальный Chrome для Windows) |
| `-ext-ver` | string | версия расширения Hola (по умолчанию определяется автоматически) |
| `-hide-SNI` | bool | скрывать SNI в TLS к Hola-агентам (по умолчанию `true`) |
| `-backoff-initial` | duration | стартовая задержка backoff'а для `zgettunnels` |
| `-backoff-deadline` | duration | общий дедлайн всех попыток `zgettunnels` |
| `-init-retries` | int | количество попыток инициализации (`0` = без ограничений) |
| `-init-retry-interval` | duration | задержка между попытками инициализации |
| `-verbosity` | int | уровень логов: 10 debug / 20 info / 30 warn / 40 error / 50 critical |
| `-version` | bool | вывести версию и выйти |

---


## Лицензия

См. [LICENSE](./LICENSE).
