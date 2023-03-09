package ripley

import (
	"fmt"
	"net/http"
	"time"

	_ "net/http/pprof"

	"github.com/VictoriaMetrics/metrics"
)

const defaultSummaryWindow = 5 * time.Minute

var defaultSummaryQuantiles = []float64{0.5, 0.9, 0.95, 0.99, 1}

func metricsServer(opts *Options) {
	if !opts.MetricsServerEnable {
		return
	}

	// Expose the registered metrics at `/metrics` path.
	http.HandleFunc("/metrics", func(w http.ResponseWriter, req *http.Request) {
		metrics.WritePrometheus(w, true)
	})

	if err := http.ListenAndServe(opts.MetricsServerAddr, nil); err != nil {
		panic(err)
	}
}

// requestDuration = metrics.NewSummaryExt(`requests_duration_seconds`, defaultSummaryWindow, defaultSummaryQuantiles)

func getOrCreateRequestDurationSummary(addr string) *metrics.Summary {
	return metrics.GetOrCreateSummaryExt(fmt.Sprintf(`requests_duration_seconds{addr="%s"}`, addr), defaultSummaryWindow, defaultSummaryQuantiles)
}

func getOrCreateResponseCodeCounter(code int, addr string) *metrics.Counter {
	return metrics.GetOrCreateCounter(fmt.Sprintf(`response_code{status="%d", addr="%s"}`, code, addr))
}

func getOrCreateFailedConnectionsCounter(addr string) *metrics.Counter {
	return metrics.GetOrCreateCounter(fmt.Sprintf(`connections_failed{addr="%s"}`, addr))
}

func getOrCreateOpenConnectionsCounter(addr string) *metrics.Counter {
	return metrics.GetOrCreateCounter(fmt.Sprintf(`connections_opened{addr="%s"}`, addr))
}

func getOrCreateClosedConnectionsCounter(addr string) *metrics.Counter {
	return metrics.GetOrCreateCounter(fmt.Sprintf(`connections_closed{addr="%s"}`, addr))
}

func getOrCreateWriteBytesCounter(addr string) *metrics.Counter {
	return metrics.GetOrCreateCounter(fmt.Sprintf(`connections_write_bytes{addr="%s"}`, addr))
}

func getOrCreateReadBytesCounter(addr string) *metrics.Counter {
	return metrics.GetOrCreateCounter(fmt.Sprintf(`connections_read_bytes{addr="%s"}`, addr))
}
