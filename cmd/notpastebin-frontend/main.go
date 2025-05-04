package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caarlos0/env"
	"github.com/joho/godotenv"
	"github.com/mayye4ka/notpastebin-frontend/internal/service"
	pbapi "github.com/mayye4ka/notpastebin/pkg/api/go"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type config struct {
	BackendAddr string `env:"BACKEND_ADDR"`
	HttpPort    int    `env:"HTTP_PORT"`
}

func readConfig() (*config, error) {
	skipEnvLoad := false
	_, err := os.Open(".env")
	if err != nil && errors.Is(err, os.ErrNotExist) {
		skipEnvLoad = true
	}
	if !skipEnvLoad {
		err = godotenv.Load()
		if err != nil {
			return nil, fmt.Errorf("load .env: %w", err)
		}
	}
	var config config
	err = env.Parse(&config)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &config, nil
}

func main() {
	termChan := make(chan os.Signal, 1)
	signal.Notify(termChan, syscall.SIGTERM)

	logger := zerolog.New(os.Stdout)
	cfg, err := readConfig()
	if err != nil {
		logger.Fatal().Err(err).Msg("can't read config")
	}

	conn, err := grpc.NewClient(cfg.BackendAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Fatal().Err(err).Msg("can't establish connection to backend")
	}
	backendClient := pbapi.NewNotPasteBinClient(conn)

	svc, err := service.New(cfg.HttpPort, backendClient, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("can't create service")
	}

	go func() {
		<-termChan
		ctx, cf := context.WithTimeout(context.Background(), time.Second*30)
		if err := svc.Shutdown(ctx); err != nil {
			logger.Fatal().Err(err).Msg("graceful shutdown failed")
		}
		cf()
	}()

	if err := svc.Start(); err != nil {
		logger.Fatal().Err(err).Msg("service failed")
	}
}
