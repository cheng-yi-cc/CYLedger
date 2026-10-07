package marketquotes

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/net/websocket"
)

// NetworkConfig contains deployment settings only, never ledger credentials.
// A relay is optional, self-hosted, and limited to public overseas market APIs.
type NetworkConfig struct {
	RelayURL   string `json:"relayUrl"`
	RelayToken string `json:"relayToken"`
}

func (c NetworkConfig) Validate() error {
	if c.RelayURL == "" {
		if c.RelayToken != "" {
			return errors.New("行情中转令牌缺少服务器地址")
		}
		return nil
	}
	u, err := url.Parse(c.RelayURL)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return errors.New("行情中转需填写不带路径、查询参数和用户信息的 HTTPS 地址")
	}
	local := net.ParseIP(u.Hostname())
	loopback := local != nil && local.IsLoopback()
	if u.Scheme != "https" && !(u.Scheme == "http" && loopback) {
		return errors.New("行情中转必须使用 HTTPS，本机回环测试除外")
	}
	if len(c.RelayToken) < 24 || len(c.RelayToken) > 256 || strings.ContainsAny(c.RelayToken, "\r\n\t ") {
		return errors.New("行情中转令牌需为 24 至 256 个非空白字符")
	}
	return nil
}

type directNetwork struct {
	handle uint64
	dns    []string
}

var platformNetwork atomic.Pointer[directNetwork]

// SetDirectNetwork is called by the Android connectivity bridge. It binds only
// market sockets, never the process, WebView, or an overseas provider connection.
func SetDirectNetwork(handle uint64, servers []string) {
	dns := make([]string, 0, 2)
	for _, server := range servers {
		if ip := net.ParseIP(server); ip != nil && len(dns) < 2 {
			dns = append(dns, ip.String())
		}
	}
	platformNetwork.Store(&directNetwork{handle: handle, dns: dns})
	Default.CloseIdleConnections()
}

func (s *Service) CloseIdleConnections() {
	s.config.HTTPClient.CloseIdleConnections()
	s.fundHTTPClient.CloseIdleConnections()
}

func newMarketHTTPClient(direct bool) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 32
	transport.MaxIdleConnsPerHost = 4
	transport.MaxConnsPerHost = 6
	transport.IdleConnTimeout = 60 * time.Second
	transport.TLSHandshakeTimeout = 3 * time.Second
	transport.ResponseHeaderTimeout = 4 * time.Second
	transport.DialContext = (&net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	if direct {
		transport.Proxy = nil
		transport.DialContext = platformDirectDial
	}
	return &http.Client{Transport: transport, Timeout: 8 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		// Never forward relay keys, query identifiers, or provider API keys elsewhere.
		if len(via) >= 3 || len(via) > 0 && (req.URL.Scheme != via[0].URL.Scheme || req.URL.Host != via[0].URL.Host) {
			return http.ErrUseLastResponse
		}
		return nil
	}}
}

func endpointSuffix(request *url.URL, endpoint string) (string, bool) {
	base, err := url.Parse(endpoint)
	if err != nil || request.Scheme != base.Scheme || request.Host != base.Host {
		return "", false
	}
	prefix := strings.TrimRight(base.Path, "/")
	if request.Path == prefix || request.Path == prefix+"/" {
		return "", true
	}
	if strings.HasPrefix(request.Path, prefix+"/") {
		return strings.TrimPrefix(request.Path, prefix), true
	}
	return "", false
}

func (s *Service) doPublicRequest(request *http.Request) (*http.Response, error) {
	for _, endpoint := range []string{s.config.FundSearchURL, s.config.FundNAVURL, s.config.TencentURL, s.config.TencentSearchURL} {
		if _, ok := endpointSuffix(request.URL, endpoint); ok {
			return s.fundHTTPClient.Do(request)
		}
	}
	if s.config.Network.RelayURL != "" {
		for _, route := range []struct{ endpoint, path string }{
			{s.config.CoinbaseRESTURL, "/v1/coinbase"},
			{s.config.CoinbaseExchangeURL, "/v1/coinbase-exchange"},
			{s.config.CoinGeckoURL, "/v1/coingecko/simple/price"},
			{s.config.CoinGeckoSearchURL, "/v1/coingecko/search"},
			{s.config.CoinGeckoCoinURL, "/v1/coingecko/coins"},
			{s.config.FXURL, "/v1/fx/usd/cny"},
			{s.config.HKDFXURL, "/v1/fx/hkd/cny"},
			{s.config.CryptoHistoryURL, "/v1/bitstamp/ohlc"},
			{s.config.HistoricalFXURL, "/v1/fx/rates"},
		} {
			if suffix, ok := endpointSuffix(request.URL, route.endpoint); ok {
				if err := s.config.Network.Validate(); err != nil {
					return nil, err
				}
				relay, _ := url.Parse(s.config.Network.RelayURL)
				relay.Path = route.path + suffix
				relay.RawQuery = request.URL.RawQuery
				clone := request.Clone(request.Context())
				clone.URL = relay
				clone.Host = ""
				clone.Header.Del("Authorization")
				clone.Header.Del("Cookie")
				clone.Header.Del("x-cg-demo-api-key")
				clone.Header.Set("X-CYLedger-Relay-Token", s.config.Network.RelayToken)
				return s.fundHTTPClient.Do(clone)
			}
		}
	}
	return s.config.HTTPClient.Do(request)
}

func (s *Service) dialMarketStream(ctx context.Context, config *websocket.Config) (*websocket.Conn, error) {
	if s.config.Network.RelayURL == "" {
		return config.DialContext(ctx)
	}
	if err := s.config.Network.Validate(); err != nil {
		return nil, err
	}
	relay, _ := url.Parse(s.config.Network.RelayURL)
	relay.Path = "/v1/coinbase-ws"
	if relay.Scheme == "https" {
		relay.Scheme = "wss"
	} else {
		relay.Scheme = "ws"
	}
	config.Location = relay
	config.Header.Set("X-CYLedger-Relay-Token", s.config.Network.RelayToken)
	work, cancel := context.WithTimeout(ctx, s.config.ReadTimeout)
	defer cancel()
	port := relay.Port()
	if port == "" {
		if relay.Scheme == "wss" {
			port = "443"
		} else {
			port = "80"
		}
	}
	conn, err := platformDirectDial(work, "tcp", net.JoinHostPort(relay.Hostname(), port))
	if err != nil {
		return nil, err
	}
	deadline, _ := work.Deadline()
	_ = conn.SetDeadline(deadline)
	rawConn := conn
	stop := context.AfterFunc(work, func() { _ = rawConn.Close() })
	if relay.Scheme == "wss" {
		secure := tls.Client(conn, &tls.Config{ServerName: relay.Hostname(), MinVersion: tls.VersionTLS12})
		if err = secure.HandshakeContext(work); err != nil {
			stop()
			conn.Close()
			return nil, err
		}
		conn = secure
	}
	ws, err := websocket.NewClient(config, conn)
	if !stop() || err != nil {
		conn.Close()
		if err != nil {
			return nil, err
		}
		return nil, work.Err()
	}
	_ = conn.SetDeadline(time.Time{})
	return ws, nil
}

// ConfigureNetworkBeforeStart is for the Android host before server startup.
// Runtime changes require a restart; request handlers must never call this method.
func (s *Service) ConfigureNetworkBeforeStart(config NetworkConfig) error {
	if err := config.Validate(); err != nil {
		return err
	}
	s.config.Network = config
	return nil
}
