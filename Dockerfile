FROM golang:1.19-bullseye

WORKDIR /app

COPY . .

RUN go mod tidy

RUN go build -o server
