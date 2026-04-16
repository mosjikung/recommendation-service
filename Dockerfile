# ---- Build stage ----
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod ./
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o /recommendation-service ./cmd/server

# ---- Run stage ----
FROM alpine:3.19

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /recommendation-service .
COPY migrations/ ./migrations/

EXPOSE 8080

CMD ["./recommendation-service"]