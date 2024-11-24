package api

import (
	"os"
	"testing"
	"time"

	db "github.com/MohammadZeyaAhmad/bank/db/sqlc"
	"github.com/MohammadZeyaAhmad/bank/util"
	"github.com/MohammadZeyaAhmad/bank/worker"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newTestServer(t *testing.T, store db.Store, taskDistributor worker.TaskDistributor) *Server {
	config := util.Config{
		TokenSymmetricKey:   util.RandomString(32),
		AccessTokenDuration: time.Minute,
	}

	server, err := NewServer(config, store, taskDistributor)
	require.NoError(t, err)

	return server
}

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)

	os.Exit(m.Run())
}
