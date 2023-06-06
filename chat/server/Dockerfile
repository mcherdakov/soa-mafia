FROM golang:1.20

WORKDIR app/

COPY . .

RUN go build -o /app/main ./server/cmd

CMD /app/main
