package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/MohammadZeyaAhmad/bank/api"
	db "github.com/MohammadZeyaAhmad/bank/db/sqlc"
	"github.com/MohammadZeyaAhmad/bank/mail"
	"github.com/MohammadZeyaAhmad/bank/worker"
	"github.com/hibiken/asynq"
	"golang.org/x/sync/errgroup"

	"github.com/MohammadZeyaAhmad/bank/util"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

var interruptSignals = []os.Signal{
	os.Interrupt,
	syscall.SIGTERM,
	syscall.SIGINT,
}

func main() {
	config, err := util.LoadConfig(".")
	if err != nil {
		log.Fatal().Err(err).Msg("cannot load config")
	}

	ctx, stop := signal.NotifyContext(context.Background(), interruptSignals...)
	defer stop()


	connPool, err := pgxpool.New(context.Background(), config.DBSource)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot connect to db")
	}

   runDBMigration(config.MigrationURL, config.DBSource)

   store := db.NewStore(connPool)
   redisOpt := asynq.RedisClientOpt{
		Addr: config.RedisAddress,
	}
  taskDistributor := worker.NewRedisTaskDistributor(redisOpt)

  waitGroup, ctx := errgroup.WithContext(ctx)

   runTaskProcessor(ctx, config, waitGroup, redisOpt, store)
   runGinServer(ctx, config, waitGroup, store, taskDistributor);

  err = waitGroup.Wait()
	if err != nil {
		log.Fatal().Err(err).Msg("error from wait group")
	}
}

func runDBMigration(migrationURL string, dbSource string) {
	migration, err := migrate.New(migrationURL, dbSource)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot create new migrate instance")
	}

	if err = migration.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatal().Err(err).Msg("failed to run migrate up")
	}

	log.Info().Msg("db migrated successfully")
}

func runTaskProcessor(ctx context.Context, config util.Config, waitGroup *errgroup.Group,  redisOpt asynq.RedisClientOpt, store db.Store) {
	mailer := mail.NewGmailSender(config.EmailSenderName, config.EmailSenderAddress, config.EmailSenderPassword)
	taskProcessor := worker.NewRedisTaskProcessor(redisOpt, store, mailer)

	log.Info().Msg("start task processor")
	err := taskProcessor.Start()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to start task processor")
	}

	waitGroup.Go(func() error {
		<-ctx.Done()
		log.Info().Msg("graceful shutdown task processor")

		taskProcessor.Shutdown()
		log.Info().Msg("task processor is stopped")

		return nil
	})
}

func runGinServer(ctx context.Context, config util.Config, 	waitGroup *errgroup.Group, store db.Store, taskDistributor worker.TaskDistributor) {
	server, err := api.NewServer(config, store, taskDistributor)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot create server")
	}

  waitGroup.Go(func() error {
    err = server.Start(config.HTTPServerAddress)
	if err != nil {
		log.Fatal().Err(err).Msg("cannot start server")
		return err
	}
	return nil;
  })

  waitGroup.Go(func() error {
		<-ctx.Done()
		log.Info().Msg("graceful shutdown HTTP server")

		err := server.Shutdown()
		if err != nil {
			log.Error().Err(err).Msg("failed to shutdown HTTP server")
			return err
		}

		log.Info().Msg("HTTP server is stopped")
		return nil
	})
	
}