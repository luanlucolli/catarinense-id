# Estágio 1: Build
FROM golang:1.26-alpine AS builder

# Define o diretório de trabalho
WORKDIR /app

# Copia os arquivos de dependências
COPY go.mod go.sum ./
RUN go mod download

# Copia o código fonte
COPY . .

# Compila a API para um binário estático (otimizado para produção)
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/api/main.go

# Estágio 2: Execução (Imagem Final)
FROM alpine:latest

# Instala certificados CA (necessário para conectar no Neon via TLS)
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copia apenas o binário compilado do estágio anterior
COPY --from=builder /app/main .

# O Render vai injetar a porta, mas deixamos a 8000 como padrão
EXPOSE 8000

# Comando para rodar a aplicação
CMD ["./main"]
