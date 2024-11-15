package main

import (
	"context"

	"github.com/MohammadZeyaAhmad/bank/api"
	db "github.com/MohammadZeyaAhmad/bank/db/sqlc"
	"github.com/MohammadZeyaAhmad/bank/mail"
	"github.com/MohammadZeyaAhmad/bank/worker"
	"github.com/hibiken/asynq"

	"github.com/MohammadZeyaAhmad/bank/util"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

func main() {
	config, err := util.LoadConfig(".")
	if err != nil {
		log.Fatal().Err(err).Msg("cannot load config")
	}

	connPool, err := pgxpool.New(context.Background(), config.DBSource)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot connect to db")
	}

   store := db.NewStore(connPool)
   redisOpt := asynq.RedisClientOpt{
		Addr: config.RedisAddress,
	}
  taskDistributor := worker.NewRedisTaskDistributor(redisOpt)
  go runTaskProcessor(config, redisOpt, store)
   server, err := api.NewServer(config, store, taskDistributor)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot create server")
	}

    
	
	err = server.Start(config.HTTPServerAddress)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot start server")
	}
}

func runTaskProcessor(config util.Config, redisOpt asynq.RedisClientOpt, store db.Store) {
	mailer := mail.NewGmailSender(config.EmailSenderName, config.EmailSenderAddress, config.EmailSenderPassword)
	taskProcessor := worker.NewRedisTaskProcessor(redisOpt, store, mailer)
	log.Info().Msg("start task processor")
	err := taskProcessor.Start()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to start task processor")
	}
}