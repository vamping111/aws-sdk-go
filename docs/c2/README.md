# Использование AWS API для C2

| Upstream Name | Upstream Version |
|------|---------|
| [aws-sdk-go](https://github.com/aws/aws-sdk-go) | [1.44.10](https://github.com/aws/aws-sdk-go/tree/v1.44.0) |

## Требования

- [Go](https://golang.org/doc/install) 1.17+

## Начало работы

Клонирование репозитория и проверка сборки модуля:

```
$ git clone git@github.com:C2Devel/aws-sdk-go.git && cd aws-sdk-go
...
$ make build
```

Артефакт в результате проверки не создается.

AWS API представлено в виде `.json` файлов в сабдиректориях `models/apis/*`.

Генерация `.go` файлов на основе API: `make generate`.

Результат генерации находится в `service/`.

Запуск линтеров: `make verify`

## Тесты

Тесты написаны с помощью [go testing](https://go.dev/doc/code#Testing).

Типы:

- unit: `make unit`
- unit + многопоточность: `make ci-test`
- интеграционные: `make integration`
    - **Важно!** Требуется доработка тестов для запуска на С2.
- unit + разные версии go: `make sandbox-tests`

## Выпуск релиза

Модуль **aws-sdk-go** используется в качестве зависимости для разных проектов,
например, [terraform-provider-rockitcloud](https://github.com/C2Devel/terraform-provider-rockitcloud).
Версия модуля указывается в `go.mod` и соответствует тегу релиза.

**Важно!** Go не позволяет изменять версии после их публикации, т.е. не допускается перестановка релизного тега.

### Версионирование

Схема версионирования: <upstream_version>-ROCKITX, где upstream_version - версия форка [aws-sdk-go](https://github.com/aws/aws-sdk-go), X - инкрементируется каждый релиз. 

Версия фиксируется только в виде тэга.

### Релиз

**Важно!** Релизы провайдера выпускаются с ветки **develop** (установлена дефолтной).
Ветка **main** используется для получения обновлений с upstream.

1. Запуск тестов (см. [тесты](#тесты))
2. Установка релизного тега с версией и его публикация

```
$ git tag v1.2.3
$ git push <remote> v1.2.3
```
