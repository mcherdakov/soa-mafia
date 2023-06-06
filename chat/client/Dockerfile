FROM golang:1.20

WORKDIR app/

COPY . .

RUN go build -o /app/main ./client/cmd

CMD /app/main
