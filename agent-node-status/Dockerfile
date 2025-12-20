# Etapa de construcción
FROM golang:1.24 AS builder

WORKDIR /app

# Copiar dependencias y descargarlas
COPY go.mod ./
COPY go.sum ./
RUN go mod download

# Copiar el resto del código fuente
COPY . ./

# Compilar el binario estático
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o agent main.go

# Etapa final mínima
FROM gcr.io/distroless/static:nonroot

WORKDIR /
COPY --from=builder /app/agent /agent

USER nonroot:nonroot
ENTRYPOINT ["/agent"]
