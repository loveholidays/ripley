package ripley

import (
	"fmt"
	"net/http"
	"time"

	_ "net/http/pprof"

	"github.com/VictoriaMetrics/metrics"
)

const metricsDefaultSummaryWindow = 5 * time.Minute

var metricsDefaultSummaryQuantiles = []float64{0.5, 0.9, 0.95, 0.99, 1}

func metricsServer(opts *Options, metricsRequestReceived chan<- bool) {
	if !opts.MetricsServerEnable {
		return
	}

	// Expose the registered metrics at `/metrics` path.
	http.HandleFunc("/metrics", func(w http.ResponseWriter, req *http.Request) {
		metrics.WritePrometheus(w, true)
		select {
		case metricsRequestReceived <- true:
		default:
		}
	})

	if err := http.ListenAndServe(opts.MetricsServerAddr, nil); err != nil {
		panic(err)
	}
}

func getOrCreatePacerPhaseTimeCounter(phase string) *metrics.Counter {
	return metrics.GetOrCreateCounter(fmt.Sprintf(`pacer_phases{phase="%s"}`, phase))
}

func getOrCreateChannelLengthCounter(name string) *metrics.Counter {
	return metrics.GetOrCreateCounter(fmt.Sprintf(`channel_length{channel="%s"}`, name))
}

func getOrCreateChannelCapacityCounter(name string) *metrics.Counter {
	return metrics.GetOrCreateCounter(fmt.Sprintf(`channel_capacity{channel="%s"}`, name))
}

func getOrCreateRequestDurationSummary(addr string) *metrics.Summary {
	return metrics.GetOrCreateSummaryExt(fmt.Sprintf(`requests_duration_seconds{addr="%s"}`, addr), metricsDefaultSummaryWindow, metricsDefaultSummaryQuantiles)
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

func updatePacerMetrics(p *phase) {
	metrics_pacer_phases := getOrCreatePacerPhaseTimeCounter(fmt.Sprintf("%s@%.3f", p.duration, p.rate))
	metrics_pacer_phases.Set(uint64(time.Now().Unix()))
}

func metricHandleResult(result *Result) {
	requests_duration_seconds := getOrCreateRequestDurationSummary(result.Request.Address)
	requests_duration_seconds.Update(result.Latency.Seconds())

	response_code := getOrCreateResponseCodeCounter(result.StatusCode, result.Request.Address)
	response_code.Inc()
}

func metricMeasureChannelCapacityAndLengh(requests chan *Request, results chan *Result) {
	ticker := time.Tick(time.Second)

	requests_channel_length := getOrCreateChannelLengthCounter("requests")
	requests_channel_capacity := getOrCreateChannelCapacityCounter("requests")

	results_channel_length := getOrCreateChannelLengthCounter("results")
	results_channel_capacity := getOrCreateChannelCapacityCounter("results")

	for range ticker {
		requests_channel_length.Set(uint64(len(requests)))
		requests_channel_capacity.Set(uint64(cap(requests)))

		results_channel_length.Set(uint64(len(results)))
		results_channel_capacity.Set(uint64(cap(results)))
	}
}
