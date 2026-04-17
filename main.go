package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

func main() {
	// 1. Carrega o arquivo .env
	// O erro é ignorado aqui porque em ambientes de produção (como Koyeb/Render),
	// as variáveis são injetadas diretamente no sistema e o arquivo .env não existirá.
	err := godotenv.Load()
	if err != nil {
		log.Println("Aviso: Arquivo .env não encontrado. Usando variáveis de ambiente do sistema.")
	}

	// 2. Pega a URL de conexão da variável de ambiente
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		log.Fatal("ERRO: Variável DATABASE_URL não definida!")
	}

	// 3. Tenta conectar ao Neon
	conn, err := pgx.Connect(context.Background(), connStr)
	if err != nil {
		log.Fatalf("Erro ao conectar ao banco: %v\n", err)
	}
	defer conn.Close(context.Background())

	fmt.Println("✅ Conexão com o Neon (São Paulo) estabelecida com sucesso!")

	// 4. Configura o servidor Web (Gin)
	r := gin.Default()

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "API da Catarinense Online!",
		})
	})

	// Pega a porta do sistema
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r.Run(":" + port)
}
