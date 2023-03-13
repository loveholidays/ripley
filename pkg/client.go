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
	"sync"
	"time"

	"github.com/valyala/fasthttp"
)

type HttpClientsPool struct {
	pool sync.Map
}

var httpClientsPool HttpClientsPool

func startClientWorkers(opts *Options, requests <-chan *request, results chan *Result) {
	go metricsServer(opts)

	ticker := time.Tick(time.Second)
	go func() {
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
	}()

	for i := 0; i < opts.NumWorkers; i++ {
		go handleResult(opts, results)
		go doHttpRequest(opts, requests, results)
	}
}

func getOrCreateHttpClient(opts *Options, req *request) (*fasthttp.HostClient, error) {
	if val, ok := httpClientsPool.pool.Load(req.Address); ok {
		return val.(*fasthttp.HostClient), nil
	}

	// If another goroutine has stored a value before us,
	// use the stored value instead of the one we just created
	val, _ := httpClientsPool.pool.LoadOrStore(req.Address, httpClientsPool.createHttpClient(opts, req))
	return val.(*fasthttp.HostClient), nil
}

func (h *HttpClientsPool) createHttpClient(opts *Options, req *request) interface{} {
	return &fasthttp.HostClient{
		Addr:                req.Address,
		Name:                "ripley",
		MaxConns:            opts.NumWorkers,
		ConnPoolStrategy:    fasthttp.LIFO,
		IsTLS:               req.IsTLS,
		MaxConnWaitTimeout:  time.Duration(opts.Timeout) * time.Second,
		MaxConnDuration:     time.Duration(opts.Timeout) * time.Second,
		MaxIdleConnDuration: time.Duration(opts.Timeout) * time.Second,
		ReadTimeout:         time.Duration(opts.Timeout) * time.Second,
		WriteTimeout:        time.Duration(opts.Timeout) * time.Second,
		Dial:                CountingDialer(opts),
	}
}
