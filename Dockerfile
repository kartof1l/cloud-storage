FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o cloud-storage ./cmd/api

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

COPY --from=builder /app/cloud-storage .
COPY --from=builder /app/web ./web

RUN mkdir -p /app/uploads

EXPOSE 8080

CMD ["./cloud-storage"]