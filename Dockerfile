# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o portico ./cmd/portico

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates docker-compose caddy
WORKDIR /root

COPY --from=builder /app/portico /usr/local/bin/portico
RUN chmod +x /usr/local/bin/portico

# Copy static files
COPY static/ /home/portico/static/

# Create portico user
RUN adduser -D -s /bin/sh portico
RUN mkdir -p /home/portico/{apps,reverse-proxy,static}
RUN chown -R portico:portico /home/portico

USER portico
WORKDIR /home/portico

ENTRYPOINT ["portico"]
