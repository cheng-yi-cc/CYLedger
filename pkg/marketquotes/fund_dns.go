package marketquotes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"sync"
	"time"
)

type referenceDial func(context.Context, string, string) (net.Conn, error)

// Only the two public fund providers prefer app-managed DNS. Connections follow
// the OS/VPN route and net/http verifies TLS against the original provider name.
func fundDNSDial(dial referenceDial, lookup func(context.Context, string) ([]string, error)) referenceDial {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, splitErr := net.SplitHostPort(address)
		if splitErr != nil || !isFundDNSHost(host) {
			return dial(ctx, network, address)
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		addresses, _ := lookup(ctx, host)
		for _, ip := range addresses {
			attempt, cancel := context.WithTimeout(ctx, 3*time.Second)
			conn, err := dial(attempt, network, net.JoinHostPort(ip, port))
			cancel()
			if err == nil || ctx.Err() != nil {
				return conn, err
			}
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		// A public resolver outage must not disable otherwise healthy system DNS.
		return dial(ctx, network, address)
	}
}

func isFundDNSHost(host string) bool {
	return host == "fundsuggest.eastmoney.com" || host == "api.fund.eastmoney.com"
}

// An IP endpoint avoids depending on the broken system resolver to reach DoH.
// Its certificate is verified normally; only the public hostname is sent.
var fundDNSClient = &http.Client{
	Timeout:       4 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

type cachedFundDNS struct {
	addresses []string
	expires   time.Time
}

type fundDNSResolver struct {
	client *http.Client
	now    func() time.Time
	mu     sync.Mutex
	cache  map[string]cachedFundDNS
}

var fundResolver = &fundDNSResolver{client: fundDNSClient, now: time.Now, cache: make(map[string]cachedFundDNS)}

func (resolver *fundDNSResolver) lookup(ctx context.Context, host string) ([]string, error) {
	if !isFundDNSHost(host) {
		return nil, errors.New("unsupported fund DNS host")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	resolver.mu.Lock()
	cached, ok := resolver.cache[host]
	resolver.mu.Unlock()
	if ok && resolver.now().Before(cached.expires) {
		return append([]string{}, cached.addresses...), nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://223.5.5.5/resolve?"+url.Values{"name": {host}, "type": {"A"}}.Encode(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/dns-json")
	response, err := resolver.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fund DNS HTTP status %d", response.StatusCode)
	}
	addresses, ttl, err := readFundDNSAddresses(response.Body)
	if err != nil {
		return nil, err
	}
	if ttl > 0 {
		resolver.mu.Lock()
		resolver.cache[host] = cachedFundDNS{addresses: append([]string{}, addresses...), expires: resolver.now().Add(ttl)}
		resolver.mu.Unlock()
	}
	return addresses, nil
}

func readFundDNSAddresses(reader io.Reader) ([]string, time.Duration, error) {
	var response struct {
		Status int
		Answer []struct {
			Type int
			Data string
			TTL  int64
		}
	}
	decoder := json.NewDecoder(io.LimitReader(reader, 16*1024))
	if err := decoder.Decode(&response); err != nil {
		return nil, 0, err
	}
	if response.Status != 0 {
		return nil, 0, errors.New("fund DNS returned no answer")
	}
	addresses := make([]string, 0, 4)
	ttl := 5 * time.Minute
	for _, record := range response.Answer {
		// Respect the whole CNAME chain and cap long-lived provider answers.
		if record.Type == 1 || record.Type == 5 {
			if record.TTL <= 0 {
				ttl = 0
			} else if record.TTL < int64(ttl/time.Second) {
				ttl = time.Duration(record.TTL) * time.Second
			}
		}
		ip, err := netip.ParseAddr(record.Data)
		if record.Type != 1 || err != nil || !ip.Is4() || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			continue
		}
		if len(addresses) < 4 {
			addresses = append(addresses, ip.String())
		}
	}
	if len(addresses) == 0 {
		return nil, 0, errors.New("fund DNS returned no public address")
	}
	return addresses, ttl, nil
}
