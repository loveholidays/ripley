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
	"encoding/json"
	"fmt"
	"math"
)

// Kbps is a download speed in kilobits per second; 0 means unlimited. In JSON
// it is a whole number, or the name of a Chrome DevTools throttling preset:
// "3g", "slow-4g" or "fast-4g". It always encodes as a number.
type Kbps int64

// presetKbps maps network presets to the download speeds of Chrome DevTools'
// throttling presets (front_end/core/sdk/NetworkManager.ts): the nominal speed
// times Chrome's real-world adjustment factor.
var presetKbps = map[string]Kbps{
	"3g":      400,  // 500 kbps × 0.8
	"slow-4g": 1440, // 1.6 Mbps × 0.9
	"fast-4g": 8100, // 9 Mbps × 0.9
}

// maxKbps is the fastest speed accepted: 10 Gbps, which is effectively
// unlimited.
const maxKbps = 10_000_000

// UnmarshalJSON reads a preset name or a whole number of kilobits per second.
// null leaves k unchanged, following the encoding/json convention for
// Unmarshalers.
func (k *Kbps) UnmarshalJSON(b []byte) error {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch v := v.(type) {
	case nil:
		return nil
	case string:
		if preset, ok := presetKbps[v]; ok {
			*k = preset
			return nil
		}
	case float64: // encoding/json decodes every number as float64
		if v == math.Trunc(v) && v >= 0 && v <= maxKbps {
			*k = Kbps(v)
			return nil
		}
	}
	return fmt.Errorf("network must be 3g, slow-4g, fast-4g or a whole number of kbps from 0 to %d, not %s", maxKbps, b)
}

const (
	bitsPerKilobit = 1000 // network speeds use decimal prefixes, not 1024
	bitsPerByte    = 8
)

// bytesPerSecond returns k in bytes per second.
func (k Kbps) bytesPerSecond() int64 {
	return int64(k) * bitsPerKilobit / bitsPerByte
}
