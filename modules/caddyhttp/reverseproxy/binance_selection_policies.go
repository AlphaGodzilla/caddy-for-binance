package reverseproxy

import (
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"strings"

	"log"
	"net/http"
	"strconv"
)

func init() {
	caddy.RegisterModule(BinanceSelection{})
}

type BinanceSelection struct {
	// Binance 每分钟最大的消耗权重，当响应头中的权重
	MaxWeight uint32 `json:"binance_max_weight,omitempty"`
	Debug     bool   `json:"debug,omitempty"`
}

// CaddyModule returns the Caddy module information.
func (BinanceSelection) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.reverse_proxy.selection_policies.binance",
		New: func() caddy.Module { return new(BinanceSelection) },
	}
}

func (b BinanceSelection) Select(pool UpstreamPool, req *http.Request, w http.ResponseWriter) *Upstream {
	if len(pool) <= 0 {
		return nil
	}
	healthPool := make(UpstreamPool, 0, len(pool))
	for _, upstream := range pool {
		if !upstream.Available() {
			continue
		}
		healthPool = append(healthPool, upstream)
	}
	if len(healthPool) <= 0 {
		return nil
	}
	upstream := SelectMinimumWeight(healthPool)
	if b.Debug {
		log.Printf("--------------- Select:  %v\n", upstream.String())
	}
	return upstream
}

func (b *BinanceSelection) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	args := d.RemainingArgs()
	var maxWeight string
	var debug string
	switch len(args) {
	case 2:
		maxWeight = args[1]
		debug = ""
		break
	case 3:
		maxWeight = args[1]
		debug = args[2]
		break
	default:
		return d.ArgErr()
	}
	maxWeight1, err := strconv.Atoi(maxWeight)
	if err != nil {
		return d.Errf("invalid MaxWeight value '%s': '%v'", maxWeight, err)
	}
	b.MaxWeight = uint32(maxWeight1)
	b.Debug = strings.EqualFold(debug, "debug")

	return nil
}

// Interface guards
var (
	_ Selector              = (*BinanceSelection)(nil)
	_ caddyfile.Unmarshaler = (*BinanceSelection)(nil)
)
