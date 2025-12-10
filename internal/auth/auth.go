package auth

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var secretKey = []byte(getSecret())

func getSecret() string {
	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		return secret
	}
	// Fallback apenas para desenvolvimento
	return "axion-insecure-secret-key-change-me"
}

// Claims customizados
type AxionClaims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// GenerateToken gera um token JWT válido por 24 horas.
func GenerateToken() (string, error) {
	claims := AxionClaims{
		Role: "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "axion-control-plane",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secretKey)
}

// ValidateToken verifica se o token é válido e retorna as claims.
func ValidateToken(tokenString string) (*AxionClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &AxionClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("método de assinatura inesperado: %v", token.Header["alg"])
		}
		return secretKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*AxionClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("token inválido")
}

// AuthMiddleware protege rotas HTTP e WebSocket.
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenString string

		// 1. Tenta pegar da Query String (Prioridade para WS/Download)
		tokenString = c.Query("token")

		// 2. Se vazio, tenta Header Authorization (Padrão API)
		if tokenString == "" {
			authHeader := c.GetHeader("Authorization")
			if authHeader != "" {
				parts := strings.Split(authHeader, " ")
				if len(parts) == 2 && parts[0] == "Bearer" {
					tokenString = parts[1]
				}
			}
		}

		if tokenString == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": "Token de autenticação necessário"})
			return
		}

		// Validar
		_, err := ValidateToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "Token inválido ou expirado"})
			return
		}

		c.Next()
	}
}

// LoginHandler processa a autenticação simples.
func LoginHandler(c *gin.Context) {
	var req struct {
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "JSON inválido"})
		return
	}

	expectedPass := os.Getenv("AXION_PASSWORD")
	if expectedPass == "" {
		expectedPass = "admin123" // Fallback inseguro
	}

	if req.Password != expectedPass {
		c.JSON(401, gin.H{"error": "Credenciais inválidas"})
		return
	}

	token, err := GenerateToken()
	if err != nil {
		c.JSON(500, gin.H{"error": "Falha ao gerar token"})
		return
	}

	c.JSON(200, gin.H{
		"token":      token,
		"expires_in": 86400,
	})
}
