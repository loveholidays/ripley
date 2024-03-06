package ripley

import (
	"net"
	"time"

	"github.com/VictoriaMetrics/metrics"
	"github.com/valyala/fasthttp"
)

type CountingConnection struct {
	net.Conn
	failedConnections *metrics.Counter
	openConnections   *metrics.Counter
	closedConnections *metrics.Counter
	writeBytes        *metrics.Counter
	readBytes         *metrics.Counter
}

func CountingDialer(opts *Options) fasthttp.DialFunc {
	return func(addr string) (net.Conn, error) {
		failedConnections := getOrCreateFailedConnectionsCounter(addr)
		openConnections := getOrCreateOpenConnectionsCounter(addr)
		closedConnections := getOrCreateClosedConnectionsCounter(addr)
		writeBytes := getOrCreateWriteBytesCounter(addr)
		readBytes := getOrCreateReadBytesCounter(addr)

		tcpDialer := &fasthttp.TCPDialer{Concurrency: opts.NumWorkers, DNSCacheDuration: 24 * time.Hour}
		conn, err := tcpDialer.DialTimeout(addr, time.Duration(opts.TimeoutConnection)*time.Second)
		if err != nil {
			failedConnections.Inc()
			return nil, err
		}

		openConnections.Inc()
		return &CountingConnection{
			conn,
			failedConnections,
			openConnections,
			closedConnections,
			writeBytes,
			readBytes,
		}, nil
	}
}

func (c *CountingConnection) Close() error {
	err := c.Conn.Close()
	c.closedConnections.Inc()
	return err
}

func (c *CountingConnection) Read(b []byte) (n int, err error) {
	n, err = c.Conn.Read(b)
	if err == nil {
		c.readBytes.Add(n)
	}

	return
}

func (c *CountingConnection) Write(b []byte) (n int, err error) {
	n, err = c.Conn.Write(b)

	if err == nil {
		c.writeBytes.Add(n)
	}

	return
}
