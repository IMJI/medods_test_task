# Тестовое задание Medods

**Улеев Данил Сергеевич**

_+7 (922) 305-01-45_

*work.uleev@yandex.ru*

### Запуск приложения

Измените значения переменных чтобы подключиться к MongoDB

```go
var port = ":3333"
var dbConnectionString = "mongodb://localhost:27017/"
var databaseName = "medods"
var collectionName = "user_refresh_tokens"
```

Запустите приложение командой

```
go run main.go
```

### Методы API

#### GET /auth?guid=_guid пользователя_

Создает пару токенов для пользователя с указанным GUID

##### Запрос

```
https://localhost:3333/auth?guid=13357205-5b97-4794-ba66-1bb603c4df61
```

##### Ответ

```json
{
  "access_token": "eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "MTMzNTcyMDUtNWI5Ny00N...c6NDQuODYyMDg="
}
```

#### POST /refresh

Обновляет пару токенов

##### Запрос

```
https://localhost:3333/auth?guid=13357205-5b97-4794-ba66-1bb603c4df61
```

##### Тело запроса

```json
{
  "access_token": "eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "MTMzNTcyMDUtNWI5Ny00N...c6NDQuODYyMDg="
}
```

##### Ответ

```json
{
  "access_token": "eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "MTMzNTcyMDUtNWI5Ny00N...A6NTguNjc1MTE="
}
```
