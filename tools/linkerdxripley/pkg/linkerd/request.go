package linkerd

type Request struct {
	ClientAddr   string `json:"client.addr"`
	ClientID     string `json:"client.id"`
	Host         string `json:"host"`
	Method       string `json:"method"`
	ProcessingNs string `json:"processing_ns"`
	RequestBytes string `json:"request_bytes"`
	Status       int    `json:"status"`
	Timestamp    string `json:"timestamp"`
	TotalNs      string `json:"total_ns"`
	TraceID      string `json:"trace_id"`
	URI          string `json:"uri"`
	UserAgent    string `json:"user_agent"`
	Version      string `json:"version"`
}