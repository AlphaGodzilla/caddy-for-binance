package reverseproxy

import (
	"fmt"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"log"
	"net/http"
	"strconv"
	"strings"
	_ "unsafe"
)

func init() {
	caddy.RegisterModule(BinanceUsageWeightInterceptor{})
	httpcaddyfile.RegisterHandlerDirective("binance_weight_interceptor", parseBinanceWeightInterceptorCaddyfile)
}

func parseBinanceWeightInterceptorCaddyfile(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	crh := new(BinanceUsageWeightInterceptor)
	err := crh.UnmarshalCaddyfile(h.Dispenser)
	if err != nil {
		return nil, err
	}
	return crh, nil
}

type BinanceUsageWeightInterceptor struct {
	WeightHeader string `json:"weight_header,omitempty"`
	Debug        bool   `json:"debug,omitempty"`
	ctx          caddy.Context
}

func (BinanceUsageWeightInterceptor) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.binance_weight_interceptor",
		New: func() caddy.Module { return new(BinanceUsageWeightInterceptor) },
	}
}

// Provision ensures that h is set up properly before use.
func (b *BinanceUsageWeightInterceptor) Provision(ctx caddy.Context) error {
	b.ctx = ctx
	updater := NewBinanceWeightMetricsUpdater(ctx, &UpstreamWeightMap)
	updater.Init()
	return nil
}

func (b *BinanceUsageWeightInterceptor) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	args := d.RemainingArgs()
	switch len(args) {
	case 2:
		b.WeightHeader = args[1]
		b.Debug = false
		break
	case 3:
		b.WeightHeader = args[1]
		b.Debug = strings.EqualFold(args[2], "debug")
	default:
		return d.ArgErr()
	}
	return nil
}

// ServeHTTP implements the Handler interface.
func (h BinanceUsageWeightInterceptor) ServeHTTP(rw http.ResponseWriter, req *http.Request, _ caddyhttp.Handler) error {
	repl := req.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
	hrc, ok := req.Context().Value(proxyHandleResponseContextCtxKey).(*handleResponseContext)

	// don't allow this to be used outside of handle_response routes
	if !ok {
		return caddyhttp.Error(http.StatusInternalServerError,
			fmt.Errorf("cannot use 'copy_response_headers' outside of reverse_proxy's handle_response routes"))
	}
	dial := hrc.dialInfo.Upstream.Dial
	for field, values := range hrc.response.Header {
		if strings.EqualFold(field, h.WeightHeader) {
			if h.Debug {
				log.Printf("%s %s = %v\n", dial, h.WeightHeader, values)
			}
			if len(values) <= 0 {
				continue
			}
			usedWeight, err := strconv.Atoi(values[0])
			if err != nil {
				return err
			}
			UpdateBinanceUsedWeight(dial, usedWeight)
			if h.Debug {
				PrintOverview()
			}
			break
		}
	}

	// make sure the reverse_proxy handler doesn't try to call
	// finalizeResponse again after we've already done it here.
	hrc.isFinalized = true

	// write the response
	return hrc.handler.finalizeResponse(rw, req, hrc.response, repl, hrc.start, hrc.logger)
}
