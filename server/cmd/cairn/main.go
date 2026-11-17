// Command cairn runs the Cairn API server.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/Indexyz/Cairn/server/internal/api"
	"github.com/Indexyz/Cairn/server/internal/notify"
	"github.com/Indexyz/Cairn/server/internal/scheduler"
	"github.com/Indexyz/Cairn/server/internal/store"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "serve" {
		usage()
		if len(os.Args) < 2 {
			os.Exit(2)
		}
		return
	}
	flags := flag.NewFlagSet("serve", flag.ExitOnError)
	host := flags.String("host", "127.0.0.1", "Bind host")
	port := flags.Int("port", 8000, "Bind port")
	dbPath := flags.String("db-path", store.DefaultPath(), "SQLite database path")
	logLevel := flags.String("log-level", "info", "Log level")
	accessLog := flags.Bool("access-log", true, "Enable access logs")
	noAccessLog := flags.Bool("no-access-log", false, "Disable access logs")
	schedulerInterval := flags.Duration("scheduler-interval", 30*time.Second, "How often to check for due schedules")
	noScheduler := flags.Bool("no-scheduler", false, "Do not fire schedules from this process")
	if err := flags.Parse(os.Args[2:]); err != nil {
		os.Exit(2)
	}
	if *noAccessLog {
		*accessLog = false
	}

	level := severity(*logLevel)
	database, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	app := &api.App{
		DB:         database,
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
		Deliverer:  &notify.Deliverer{Client: &http.Client{Timeout: 10 * time.Second}},
		AccessLog:  *accessLog,
		Logger:     func(format string, args ...any) { log.Printf(format, args...) },
	}
	server := &http.Server{
		Addr:              net.JoinHostPort(*host, strconv.Itoa(*port)),
		Handler:           app.Handler(),
		ReadHeaderTimeout: 15 * time.Second,
	}
	if level > 0 {
		log.SetFlags(log.LstdFlags)
	}
	log.Printf("Cairn %s listening on %s", api.Version, server.Addr)

	stop := make(chan os.Signal, 1)
	schedulerCtx, stopScheduler := context.WithCancel(context.Background())
	defer stopScheduler()
	if !*noScheduler {
		runner := &scheduler.Runner{
			DB:       database,
			Interval: *schedulerInterval,
			Notify:   app.NotifyProjectCreated,
			Log:      func(format string, args ...any) { log.Printf(format, args...) },
		}
		go runner.Run(schedulerCtx)
	}
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		stopScheduler()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server failed: %v", err)
	}
}

func severity(raw string) int {
	switch raw {
	case "debug", "DEBUG":
		return 1
	default:
		return 0
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage: cairn serve [OPTIONS]")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "  Cairn - Fact-graph based collaborative exploration protocol.")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  serve  Start the Cairn API server.")
}
