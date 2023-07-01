package reverseproxy

import (
	"github.com/caddyserver/caddy/v2"
	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
	"runtime/debug"
	"sync"
	"time"
)

var binanceWeightMetrics = struct {
	init          sync.Once
	BinanceWeight *prometheus.GaugeVec
}{}

func initBinanceWeightMetrics() {
	const ns, sub = "custom", "binance"

	upstreamsLabels := []string{"upstream"}
	binanceWeightMetrics.BinanceWeight = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: ns,
		Subsystem: sub,
		Name:      "upstream_used_weight",
		Help:      "上游Binance当前已使用的权重",
	}, upstreamsLabels)
}

type BinanceWeightMetricsUpdater struct {
	ctx                caddy.Context
	upstreamWeightPool *cmap.ConcurrentMap[string, *UpstreamWeight]
}

func NewBinanceWeightMetricsUpdater(ctx caddy.Context, weightPool *cmap.ConcurrentMap[string, *UpstreamWeight]) *BinanceWeightMetricsUpdater {
	binanceWeightMetrics.init.Do(func() {
		initBinanceWeightMetrics()
	})
	binanceWeightMetrics.BinanceWeight.Reset()
	return &BinanceWeightMetricsUpdater{ctx: ctx, upstreamWeightPool: weightPool}
}

func (s *BinanceWeightMetricsUpdater) Init() {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				reverseProxyMetrics.logger.Error("upstreams binance weight metrics updater panicked",
					zap.Any("error", err),
					zap.ByteString("stack", debug.Stack()))
			}
		}()

		s.update()

		ticker := time.NewTicker(10 * time.Second)
		for {
			select {
			case <-ticker.C:
				s.update()
			case <-s.ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}

func (s *BinanceWeightMetricsUpdater) update() {
	for value := range s.upstreamWeightPool.IterBuffered() {
		labels := prometheus.Labels{"upstream": value.Val.Id}
		value.Val.ReadWeight()
		gaugeValue := float64(value.Val.ReadWeight())
		binanceWeightMetrics.BinanceWeight.With(labels).Set(gaugeValue)
	}
}
