# Multi-stage build para o controller
FROM golang:1.21-alpine AS builder
RUN apk add --no-cache git ca-certificates
WORKDIR /workspace

# Copia módulos primeiro para melhor cache
COPY go.mod go.sum ./
RUN go mod download

# Copia o restante do código e compila binário estático
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} go build -ldflags="-s -w" -o manager ./cmd/manager

# Imagem final mínima
FROM gcr.io/distroless/static-debian12
WORKDIR /
COPY --from=builder /workspace/manager /manager
USER 65532:65532
ENTRYPOINT ["/manager"]
