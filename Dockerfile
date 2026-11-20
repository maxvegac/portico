# Build stage
FROM golang:1.21-alpine AS builder

ARG VERSION=dev

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Static files and templates are embedded in the binary
RUN CGO_ENABLED=0 go build -ldflags "-X main.version=${VERSION}" -o portico ./src/cmd/portico

# Runtime stage
FROM alpine:3.20

RUN apk --no-cache add ca-certificates docker-cli docker-cli-compose git

COPY --from=builder /app/portico /usr/local/bin/portico

# Create portico user
RUN adduser -D -s /bin/sh portico \
    && mkdir -p /home/portico/apps /home/portico/reverse-proxy /home/portico/www \
    && chown -R portico:portico /home/portico

USER portico
WORKDIR /home/portico

ENTRYPOINT ["portico"]
