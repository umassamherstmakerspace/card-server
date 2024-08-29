FROM golang:1.21-alpine

WORKDIR /app

COPY go.mod ./
COPY go.sum ./

RUN go mod download

COPY ./src/ ./src

RUN go build -o /server src/server.go

EXPOSE 3000

CMD [ "/server" ]