FROM golang:1.26.1-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/gophprofile-server ./cmd/server/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/gophprofile-worker ./cmd/worker/main.go

FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates curl

COPY --from=builder /app/gophprofile-server /app/gophprofile-server
COPY --from=builder /app/gophprofile-worker /app/gophprofile-worker
COPY ./configs /app/configs
COPY ./web /app/web
COPY ./migrations /app/migrations

EXPOSE 8080

CMD ["/app/gophprofile-server"]