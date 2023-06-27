package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/urfave/cli/v2"
)

var (
	closeTimeout = time.Second
	upgrader     = websocket.Upgrader{}
)

func main() {
	ctx := context.Background()
	logger := zerolog.New(os.Stdout).
		Level(zerolog.InfoLevel).
		With().
		Timestamp().
		Logger()
	ctx = logger.WithContext(ctx)

	shutdown := make(chan struct{})
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()
	go func() {
		<-ctx.Done()
		close(shutdown)
	}()

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
			logger.Debug().Msg("Debug logging enabled")

			route := strings.TrimSuffix(c.String("route"), "/")

			ticker := &ticker{
				logger:   logger,
				route:    route,
				interval: c.Duration("interval"),
				shutdown: shutdown,
			}

			logger.Info().
				Str("route", route).
				Uint("port", c.Uint("port")).
				Msgf("Listening on port %d", c.Uint("port"))

			errCh := make(chan error)
			go func() {
				errCh <- http.ListenAndServe(fmt.Sprintf(":%d", c.Uint("port")), ticker)
			}()

			for {
				select {
				case <-ctx.Done():
					return nil
				case err := <-errCh:
					return err
				}
			}
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
	shutdown chan struct{}
}

func (t *ticker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logger := t.logger.With().Str("method", r.Method).Str("uri", r.RequestURI).Logger()
	logger.Info().Msg("Received request")
	headersEvt := logger.Debug()
	for k, v := range r.Header {
		headersEvt = headersEvt.Strs(k, v)
	}
	headersEvt.Msg("Request headers")

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

		case <-t.shutdown:
			logger.Info().Err(err).Msg("Shutting down")
			if err := conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
				time.Now().Add(closeTimeout),
			); err != nil {
				logger.Warn().Err(err).Msg("Failed to write close message")
			}
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
