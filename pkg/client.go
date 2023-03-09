/*
ripley
Copyright (C) 2021  loveholidays

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package ripley

import (
	"net/url"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
)

var (
	// pool of PipelineClient instances, indexed by host address
	clientsPool sync.Map
)

func startClientWorkers(opts Options, requests <-chan *request, results chan *Result) {
	go metricsServer(opts)

	for i := 0; i < opts.NumWorkers; i++ {
		go doHttpRequest(opts, requests, results)
		go handleResult(opts, results)
	}
}

func getOrCreateHttpClient(opts Options, req *request) (*fasthttp.HostClient, error) {
	// parse URL to get host address
	up, err := url.Parse(req.Url)
	if err != nil {
		return nil, err
	}

	if up.Port() == "" {
		up.Host = up.Host + ":80"
	}

	// check if a PipelineClient instance is already available in the pool
	if val, ok := clientsPool.Load(up.Host); ok {
		if client, ok := val.(*fasthttp.HostClient); ok {
			return client, nil
		}
	}

	// create a new PipelineClient instance
	client := &fasthttp.HostClient{
		Addr:                up.Host,
		Name:                "ripley",
		MaxConns:            opts.NumWorkers,
		ConnPoolStrategy:    fasthttp.LIFO,
		IsTLS:               up.Scheme == "https",
		MaxConnWaitTimeout:  time.Duration(opts.Timeout) * time.Second,
		MaxConnDuration:     time.Duration(opts.Timeout) * time.Second,
		MaxIdleConnDuration: time.Duration(opts.Timeout) * time.Second,
		ReadTimeout:         time.Duration(opts.Timeout) * time.Second,
		WriteTimeout:        time.Duration(opts.Timeout) * time.Second,
		Dial:                CountingDialer(opts),
	}

	// add the new PipelineClient instance to the pool
	clientsPool.Store(up.Host, client)

	return client, nil
}
