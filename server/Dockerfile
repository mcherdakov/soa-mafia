FROM golang:1.20

WORKDIR app/

COPY . .

RUN go build -o /app/main ./server/cmd/server

CMD /app/main
