package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/urfave/cli/v2"
)

var upgrader = websocket.Upgrader{}

func main() {
	ctx := context.Background()
	logger := zerolog.New(os.Stdout).
		Level(zerolog.InfoLevel).
		With().
		Timestamp().
		Logger()
	ctx = logger.WithContext(ctx)

	app := &cli.App{
		Name: "ws-ticker",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "debug",
				Usage:   "enable debug logging",
				EnvVars: []string{"DEBUG"},
			},
			&cli.StringFlag{
				Name:    "route",
				Usage:   "route to listen to",
				Value:   "/ticker",
				EnvVars: []string{"ROUTE"},
			},
			&cli.DurationFlag{
				Name:    "interval",
				Usage:   "interval between ticks",
				Value:   1 * time.Second,
				EnvVars: []string{"INTERVAL"},
			},
			&cli.UintFlag{
				Name:    "port",
				Usage:   "port to listen on",
				Value:   8080,
				EnvVars: []string{"PORT"},
			},
		},
		Action: func(c *cli.Context) error {
			if c.Bool("debug") {
				logger = logger.Level(zerolog.DebugLevel)
			}
			route := strings.TrimSuffix(c.String("route"), "/")

			ticker := &ticker{
				logger:   logger,
				route:    route,
				interval: c.Duration("interval"),
			}

			logger.Info().Msgf("Listening on port %d", c.Uint("port"))

			return http.ListenAndServe(fmt.Sprintf(":%d", c.Uint("port")), ticker)
		},
	}

	if err := app.RunContext(ctx, os.Args); err != nil {
		logger.Err(err).Msg("Failed to run app")
	} else {
		logger.Info().Msg("App finished")
	}
}

type ticker struct {
	logger   zerolog.Logger
	route    string
	interval time.Duration
}

func (t *ticker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := t.logger.With().Str("method", r.Method).Str("uri", r.RequestURI).Logger()
	logger.Info().Msg("Received request")

	if path := strings.TrimSuffix(r.URL.Path, "/"); path != t.route {
		logger.Warn().Str("path", path).Msg("Invalid route")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		t.logger.Err(err).Msg("Failed to upgrade connection")
		return
	}
	defer conn.Close()

	ticker := time.NewTicker(t.interval)

	for {
		select {
		case tick := <-ticker.C:
			logger.Debug().Msg("Ticking")
			if err := t.writeMessage(conn, logger, fmt.Sprintf("Tick at %s", tick.Format(time.RFC3339))); err != nil {
				return
			}

		case <-r.Context().Done():
			err := r.Context().Err()
			logger.Warn().Err(err).Msg("Context done")
			return
		}
	}
}

func (t *ticker) writeMessage(conn *websocket.Conn, logger zerolog.Logger, msg string) error {
	w, err := conn.NextWriter(websocket.TextMessage)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to get writer")
		return err
	}

	if _, err := w.Write([]byte(msg + "\n")); err != nil {
		logger.Warn().Err(err).Msg("Failed to write message")
		return err
	}

	if err := w.Close(); err != nil {
		logger.Warn().Err(err).Msg("Failed to close writer")
		return err
	}

	return nil
}
