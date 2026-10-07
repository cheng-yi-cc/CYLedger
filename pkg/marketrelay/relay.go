// Package marketrelay exposes only allowlisted public market data. It has no
// database, account endpoints, trading permissions, or arbitrary destination URL.
package marketrelay

import (
	"context"
	"crypto/subtle"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

const TokenHeader = "X-CYLedger-Relay-Token"
const maxBody = 4 * 1024 * 1024

type Config struct {
	Token           string
	CoinGeckoAPIKey string
	Client          *http.Client
}

type cachedResponse struct {
	body        []byte
	contentType string
	expires     time.Time
}
type Relay struct {
	config    Config
	cache     map[string]cachedResponse
	mu        sync.Mutex
	flights   singleflight.Group
	slots     chan struct{}
	upstreams chan struct{}
	sockets   chan struct{}
}

func New(config Config) (*Relay, error) {
	if len(config.Token) < 24 || len(config.Token) > 256 || strings.ContainsAny(config.Token, "\r\n\t ") {
		return nil, errors.New("MARKET_RELAY_TOKEN must contain 24-256 non-whitespace characters")
	}
	if config.Client == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = nil
		transport.DialContext = (&net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}).DialContext
		transport.TLSHandshakeTimeout = 3 * time.Second
		transport.ResponseHeaderTimeout = 5 * time.Second
		transport.MaxConnsPerHost = 12
		transport.MaxIdleConnsPerHost = 6
		config.Client = &http.Client{Transport: transport, Timeout: 8 * time.Second}
	}
	// A trusted endpoint can still redirect to an untrusted one; never follow it.
	client := *config.Client
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	config.Client = &client
	return &Relay{config: config, cache: make(map[string]cachedResponse), slots: make(chan struct{}, 12), upstreams: make(chan struct{}, 12), sockets: make(chan struct{}, 16)}, nil
}

var productPath = regexp.MustCompile(`^/products/[A-Z0-9][A-Z0-9._-]{0,63}(/ticker)?$`)
var coinPath = regexp.MustCompile(`^/coins/[a-z0-9][a-z0-9._-]{0,63}$`)
var candlePath = regexp.MustCompile(`^/products/[A-Z0-9][A-Z0-9._-]{0,63}/candles$`)
var ohlcPath = regexp.MustCompile(`^/coins/[a-z0-9][a-z0-9._-]{0,63}/ohlc$`)
var bitstampPath = regexp.MustCompile(`^/v1/bitstamp/ohlc/(btc|eth|sol|usdt|usdc)usd/$`)

// ResolveTarget never uses a caller-supplied host, scheme, port, credentials or
// path suffix outside a precise public endpoint. Unknown query fields are rejected.
func ResolveTarget(u *url.URL) (string, time.Duration, error) {
	if u.RawPath != "" || len(u.RawQuery) > 4096 || strings.Contains(u.Path, "..") {
		return "", 0, errors.New("invalid path")
	}
	path := u.Path
	base := ""
	allowed := ""
	ttl := time.Minute
	switch {
	case path == "/v1/coinbase/products":
		base = "https://api.coinbase.com/api/v3/brokerage/market/products"
		allowed = "limit product_type product_ids"
		ttl = 5 * time.Minute
	case strings.HasPrefix(path, "/v1/coinbase/") && productPath.MatchString(strings.TrimPrefix(path, "/v1/coinbase")):
		base = "https://api.coinbase.com/api/v3/brokerage/market" + strings.TrimPrefix(path, "/v1/coinbase")
		allowed = "limit"
		ttl = 5 * time.Second
	case path == "/v1/coingecko/simple/price":
		base = "https://api.coingecko.com/api/v3/simple/price"
		allowed = "ids vs_currencies include_last_updated_at include_24hr_change precision"
		ttl = time.Minute
	case strings.HasPrefix(path, "/v1/coinbase/") && candlePath.MatchString(strings.TrimPrefix(path, "/v1/coinbase")):
		base = "https://api.coinbase.com/api/v3/brokerage/market" + strings.TrimPrefix(path, "/v1/coinbase")
		allowed = "start end granularity limit"
	case strings.HasPrefix(path, "/v1/coinbase-exchange/") && candlePath.MatchString(strings.TrimPrefix(path, "/v1/coinbase-exchange")):
		base = "https://api.exchange.coinbase.com" + strings.TrimPrefix(path, "/v1/coinbase-exchange")
		allowed = "start end granularity"
	case strings.HasPrefix(path, "/v1/coingecko/") && ohlcPath.MatchString(strings.TrimPrefix(path, "/v1/coingecko")):
		base = "https://api.coingecko.com/api/v3" + strings.TrimPrefix(path, "/v1/coingecko")
		allowed = "vs_currency days precision"
	case bitstampPath.MatchString(path):
		base = "https://www.bitstamp.net/api/v2/ohlc" + strings.TrimPrefix(path, "/v1/bitstamp/ohlc")
		allowed = "step start end limit"
	case path == "/v1/fx/rates":
		base = "https://api.frankfurter.dev/v2/providers/ecb/rates"
		allowed = "base quotes date"
		ttl = time.Hour
	case path == "/v1/coingecko/search":
		base = "https://api.coingecko.com/api/v3/search"
		allowed = "query"
		ttl = 10 * time.Minute
	case strings.HasPrefix(path, "/v1/coingecko/") && coinPath.MatchString(strings.TrimPrefix(path, "/v1/coingecko")):
		base = "https://api.coingecko.com/api/v3" + strings.TrimPrefix(path, "/v1/coingecko")
		allowed = "localization tickers market_data community_data developer_data"
		ttl = time.Hour
	case path == "/v1/fx/usd/cny" || path == "/v1/fx/hkd/cny":
		base = "https://api.frankfurter.dev/v2/providers/ecb/rate/" + strings.TrimPrefix(path, "/v1/fx/")
		allowed = "date"
		ttl = time.Hour
	default:
		return "", 0, errors.New("unsupported public market endpoint")
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return "", 0, err
	}
	for key, values := range query {
		if key == "" || strings.ContainsAny(key, " \t\r\n") || !strings.Contains(" "+allowed+" ", " "+key+" ") || len(values) > 100 {
			return "", 0, errors.New("unsupported query")
		}
		for _, value := range values {
			if len(value) > 2048 {
				return "", 0, errors.New("query too large")
			}
		}
	}
	if q := query.Encode(); q != "" {
		base += "?" + q
	}
	return base, ttl, nil
}

func (s *Relay) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", 405)
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.Header.Get(TokenHeader)), []byte(s.config.Token)) != 1 {
		http.Error(w, "unauthorized", 401)
		return
	}
	// Neither application cookies nor account tokens are accepted by this service.
	if r.ContentLength != 0 || len(r.TransferEncoding) > 0 || r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
		http.Error(w, "public data requests only", 400)
		return
	}
	if r.URL.Path == "/v1/coinbase-ws" {
		if r.URL.RawQuery != "" {
			http.Error(w, "invalid websocket route", 400)
			return
		}
		s.serveSocket(w, r)
		return
	}
	target, ttl, err := ResolveTarget(r.URL)
	if err != nil {
		http.Error(w, "unsupported public market endpoint", 400)
		return
	}
	s.mu.Lock()
	cached, ok := s.cache[target]
	s.mu.Unlock()
	if ok && time.Now().Before(cached.expires) {
		writeResponse(w, cached, "HIT")
		return
	}
	// Cap total waiting work, not just distinct upstream requests.
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	default:
		w.Header().Set("Retry-After", "2")
		http.Error(w, "busy", 429)
		return
	}
	result := s.flights.DoChan(target, func() (interface{}, error) {
		s.mu.Lock()
		cached, ok := s.cache[target]
		s.mu.Unlock()
		if ok && time.Now().Before(cached.expires) {
			return cached, nil
		}
		// Caller cancellation releases its waiting slot, not the shared upstream
		// slot. Otherwise rapid disconnects can exceed the actual work limit.
		select {
		case s.upstreams <- struct{}{}:
			defer func() { <-s.upstreams }()
		default:
			return nil, upstreamError{http.StatusTooManyRequests}
		}
		work, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		req, _ := http.NewRequestWithContext(work, http.MethodGet, target, nil)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "CYLedger-MarketRelay/1.0")
		if strings.HasPrefix(target, "https://api.coingecko.com/") && s.config.CoinGeckoAPIKey != "" {
			req.Header.Set("x-cg-demo-api-key", s.config.CoinGeckoAPIKey)
		}
		response, err := s.config.Client.Do(req)
		if err != nil {
			return nil, errors.New("upstream unavailable")
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return nil, upstreamError{response.StatusCode}
		}
		body, err := io.ReadAll(io.LimitReader(response.Body, maxBody+1))
		if err != nil || len(body) > maxBody {
			return nil, errors.New("invalid upstream payload")
		}
		cached = cachedResponse{body: body, contentType: "application/json", expires: time.Now().Add(ttl)}
		s.mu.Lock()
		// Bound bytes as well as entry count (large metadata is not cached).
		if len(body) <= 128*1024 {
			if len(s.cache) >= 128 {
				for key := range s.cache {
					delete(s.cache, key)
					break
				}
			}
			s.cache[target] = cached
		}
		s.mu.Unlock()
		return cached, nil
	})
	select {
	case <-r.Context().Done():
		return
	case result := <-result:
		if result.Err != nil {
			var upstream upstreamError
			if errors.As(result.Err, &upstream) && upstream.status == 429 {
				w.Header().Set("Retry-After", "60")
				http.Error(w, "upstream rate limited", 429)
			} else {
				http.Error(w, "upstream unavailable", 502)
			}
			return
		}
		writeResponse(w, result.Val.(cachedResponse), "MISS")
	}
}

type upstreamError struct{ status int }

func (upstreamError) Error() string { return "upstream unavailable" }
func writeResponse(w http.ResponseWriter, cached cachedResponse, state string) {
	w.Header().Set("Content-Type", cached.contentType)
	w.Header().Set("X-CYLedger-Cache", state)
	_, _ = w.Write(cached.body)
}

// Only the Coinbase public market stream can be upgraded. No generic CONNECT,
// or arbitrary WS targets are accepted. Client frames are not reinterpreted;
// this route is not a shared subscription broker or a trading service.
func (s *Relay) serveSocket(w http.ResponseWriter, r *http.Request) {
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		http.Error(w, "websocket required", 400)
		return
	}
	select {
	case s.sockets <- struct{}{}:
		defer func() { <-s.sockets }()
	default:
		http.Error(w, "too many streams", 429)
		return
	}
	proxy := &httputil.ReverseProxy{
		Rewrite: func(p *httputil.ProxyRequest) {
			p.Out.URL.Scheme = "https"
			p.Out.URL.Host = "advanced-trade-ws.coinbase.com"
			p.Out.URL.Path = "/"
			p.Out.URL.RawPath = ""
			p.Out.URL.RawQuery = ""
			p.Out.Host = "advanced-trade-ws.coinbase.com"
			// Forward only the WebSocket handshake, never arbitrary client headers.
			headers := make(http.Header)
			for _, key := range []string{"Connection", "Upgrade", "Sec-WebSocket-Key", "Sec-WebSocket-Version", "Sec-WebSocket-Protocol", "Sec-WebSocket-Extensions"} {
				if values := p.Out.Header.Values(key); len(values) > 0 {
					headers[http.CanonicalHeaderKey(key)] = append([]string{}, values...)
				}
			}
			p.Out.Header = headers
			p.Out.Header.Set("Origin", "https://www.coinbase.com")
		},
		Transport:    s.config.Client.Transport,
		ErrorLog:     log.New(io.Discard, "", 0),
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) { http.Error(w, "stream unavailable", 502) },
	}
	proxy.ServeHTTP(w, r)
}
