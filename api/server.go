package api

import (
	"fmt"
	"log"
	"net/http"

	db "github.com/MohammadZeyaAhmad/bank/db/sqlc"
	"github.com/MohammadZeyaAhmad/bank/token"
	"github.com/MohammadZeyaAhmad/bank/util"
	"github.com/MohammadZeyaAhmad/bank/worker"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

// Server serves HTTP requests for our banking service.
type Server struct {
	config          util.Config
	store           db.Store
	tokenMaker      token.Maker
	router          *gin.Engine
	taskDistributor worker.TaskDistributor
	httpServer      *http.Server
}

// NewServer creates a new HTTP server and set up routing.
func NewServer(config util.Config, store db.Store, taskDistributor worker.TaskDistributor) (*Server, error) {
	tokenMaker, err := token.NewPasetoMaker(config.TokenSymmetricKey)
	if err != nil {
		return nil, fmt.Errorf("cannot create token maker: %w", err)
	}

	server := &Server{
		config:          config,
		store:           store,
		tokenMaker:      tokenMaker,
		taskDistributor: taskDistributor,
	}

	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		if err := v.RegisterValidation("currency", validCurrency); err != nil {
			log.Fatalf("Error registering validation: %v", err)
		}
	}
	server.setupRouter()

	server.httpServer = &http.Server{
		Handler: server.router,
	}
	return server, nil
}

func (server *Server) setupRouter() {
	router := gin.Default()

	router.POST("/users", server.createUser)
	router.POST("/users/login", server.loginUser)
	router.POST("/tokens/renew_access", server.renewAccessToken)

	authRoutes := router.Group("/").Use(authMiddleware(server.tokenMaker))
	authRoutes.POST("/accounts", server.createAccount)
	authRoutes.GET("/accounts/:id", server.getAccount)
	authRoutes.GET("/accounts", server.listAccounts)
	authRoutes.POST("/transfers", server.createTransfer)
	server.router = router
}

// Start runs the HTTP server on a specific address.
func (server *Server) Start(address string) error {
	return server.router.Run(address)
}

func (server *Server) Shutdown() error {
	if server.httpServer == nil {
		err := fmt.Errorf("server is not initialized")
		return err
	}

	fmt.Println("Shutting down the server...")
	if err := server.httpServer.Close(); err != nil {
		return err
	}

	fmt.Println("Server shutdown completed successfully.")
	return nil
}

func errorResponse(err error) gin.H {
	return gin.H{"error": err.Error()}
}
