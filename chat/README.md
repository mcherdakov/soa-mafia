# soa-chat

Для запуска сервера необходимо ввести команду:

```bash
make run-server
```

Далее можно запусить несколько клиентов с помощью команды:

```bash
make run-client
```

Нужно будет ввести имя пользователя и сессию. Клиенты с одинаковым значением сессии будут получать сообщения друг друга.


В качестве очереди сообщений используется RabbitMQ, под каждую сессию заводится отдельный exchange, под каждого клиента отдельная очередь. Для взаимодействия сервера с клиентом используется grpc.
