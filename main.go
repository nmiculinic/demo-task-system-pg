package main

import (
	"context"
	"math/rand"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/log/zapadapter"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

var (
	readyTasks = make(chan uuid.UUID, 1000)
	rateLimit = make(chan struct{}, 1000)
	pool *pgxpool.Pool
	notifCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "myapp_notif_recv_total",
		Help: "Total number of pg notification recieved",
	})
	addedProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "myapp_added_ops_total",
		Help: "The total number of added events",
	})
	addedDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:        "myapp_added_ops_seconds",
		Help:        "The total duration of added events",
		Buckets:     prometheus.DefBuckets,
	})
	opsProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "myapp_processed_ops_total",
		Help: "The total number of processed events",
	})
	totalProcessed int64
	opsLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:        "myapp_processed_ops_seconds",
		Help:        "The total duration of processed events",
		Buckets:     prometheus.DefBuckets,
	})
	throughPut = promauto.NewGauge(prometheus.GaugeOpts{
		Name:        "myapp_processed_throughput",
		Help:        "ops throughput",
	})
	rateLimitGuage = promauto.NewGaugeFunc(prometheus.GaugeOpts{
		Name:        "myapp_ratelimit_length",
		Help:        "ratelimit length",
	}, func() float64 {
		return float64(len(rateLimit))
	})
)

func producer(ctx context.Context) error {
	for {
		rateLimit <- struct{}{}
		if _, err := genFn(ctx, 3); err != nil {
			return err
		}
	}

}

func genFn(ctx context.Context, depth int) (*uuid.UUID, error) {
	t := time.Now()
	id := uuid.New()
	deps := make([]uuid.UUID, 0)

	numDep := int(rand.Int31n(4))
	if depth >= 3 {
		numDep = 0
	}

	for i := 0; i < numDep; i++ {
		u, err := genFn(ctx, depth + 1)
		if err != nil {
			return nil, err
		}
		deps = append(deps, *u)
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()

	_, err = conn.Exec(ctx, `
		INSERT INTO tasks(id, data, deps) 
		VALUES ($1, '{}', $2)
	`, id, deps)
	if err != nil {
		return nil, err
	}
	addedProcessed.Inc()
	addedDuration.Observe(float64(time.Since(t))/ float64(time.Second))
	return &id, nil
}

func initialLoad(ctx context.Context) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	rows, err := conn.Query(ctx, `
SELECT id, data
FROM tasks
WHERE finished = FALSE AND deps = '{}'
`)
	if err != nil {
		conn.Release()
		return err
	}
	for rows.Next() {
		taskId := uuid.UUID{}
		data := ""
		if err := rows.Scan(&taskId, &data); err != nil {
			conn.Release()
			return err
		}
		readyTasks <- taskId
	}
	zap.L().Info("initial load added to the queue")
	return nil
}

func worker(ctx context.Context) error {
	for taskId := range readyTasks {
		<-rateLimit
		if err := processEntry(ctx, taskId); err != nil {
			return err
		}
	}
	return nil
}

func watchReadyTasks(ctx context.Context) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Conn().Exec(ctx, `LISTEN ready_task`); err != nil {
		return err
	}
	for {
		notif, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			return err
		}
		notifCount.Inc()
		readyTasks <- uuid.MustParse(notif.Payload)
	}

}

func processEntry(ctx context.Context, id uuid.UUID) error {
	t := time.Now()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
UPDATE tasks
SET finished = TRUE
WHERE id = $1`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
UPDATE tasks
SET deps = array_remove(deps, $1::uuid)
WHERE ARRAY[$1::uuid] <@ deps
				`, id); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	opsProcessed.Inc()
	atomic.AddInt64(&totalProcessed, 1)
	opsLatency.Observe(float64(time.Since(t)) / float64(time.Second))
	return nil
}

func setup() error {
	logger, err := zap.NewDevelopment()
	if err != nil {
		return err
	}
	zap.ReplaceGlobals(logger)
	zap.RedirectStdLog(logger)

	pgxConfig, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	pgxConfig.ConnConfig.Logger = zapadapter.NewLogger(zap.L())
	pgxConfig.ConnConfig.LogLevel = pgx.LogLevelWarn

	ctx := context.TODO()
	pool, err = pgxpool.ConnectConfig(ctx, pgxConfig)
	if err != nil {
		return err
	}
	return nil
}

func throughput() {
	for {
		t := time.Now()
		time.Sleep(5 * time.Second)
		total := atomic.LoadInt64(&totalProcessed)
		val := float64(total) /
				(float64(time.Since(t)) / float64(time.Second))
		throughPut.Set(val)
		zap.L().Info("throughput ticker", zap.Float64("it/s", val))
		atomic.StoreInt64(&totalProcessed, 0)
	}
}

func main() {
	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(":6060", nil)

	ctx := context.TODO()
	if err := setup(); err != nil {
		panic(err)
	}

	logger := zap.L()
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		logger.Info("starting watch for notifications")
		return watchReadyTasks(ctx)
	})
	g.Go(func() error {
		logger.Info("Starting producer")
		return producer(ctx)
	})
	g.Go(func() error {
		logger.Info("Initial load")
		return initialLoad(ctx)
	})
	g.Go(func() error {
		logger.Info("Starting worker")
		return worker(ctx)
	})
	g.Go(func() error {
		throughput()
		return nil
	})
	if err := g.Wait(); err != nil {
		panic(err)
	}
}
