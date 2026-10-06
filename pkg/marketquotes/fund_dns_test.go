package marketquotes

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFundDNSPriorityAndScope(t *testing.T) {
	for _, test := range []struct {
		name, host string
		managed    bool
	}{
		{"fund search", "fundsuggest.eastmoney.com", true},
		{"fund yield", "api.fund.eastmoney.com", true},
		{"crypto unchanged", "api.coinbase.com", false},
		{"unrelated host unchanged", "example.com", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			lookups := 0
			var addresses []string
			client, server := net.Pipe()
			defer client.Close()
			defer server.Close()
			dial := fundDNSDial(func(_ context.Context, _, address string) (net.Conn, error) {
				addresses = append(addresses, address)
				return client, nil
			}, func(_ context.Context, host string) ([]string, error) {
				lookups++
				require.Equal(t, test.host, host)
				return []string{"103.220.165.216"}, nil
			})
			conn, err := dial(context.Background(), "tcp", test.host+":443")
			require.NoError(t, err)
			require.Same(t, client, conn)
			if test.managed {
				require.Equal(t, 1, lookups)
				require.Equal(t, []string{"103.220.165.216:443"}, addresses)
			} else {
				require.Zero(t, lookups)
				require.Equal(t, []string{test.host + ":443"}, addresses)
			}
		})
	}
}

func TestFundDNSFallsBackWhenResolverOrAddressesFail(t *testing.T) {
	for _, resolverFails := range []bool{false, true} {
		var addresses []string
		client, server := net.Pipe()
		dial := fundDNSDial(func(_ context.Context, _, address string) (net.Conn, error) {
			addresses = append(addresses, address)
			if address == "fundsuggest.eastmoney.com:443" {
				return client, nil
			}
			return nil, errors.New("address unreachable")
		}, func(context.Context, string) ([]string, error) {
			if resolverFails {
				return nil, errors.New("resolver unavailable")
			}
			return []string{"103.220.165.216"}, nil
		})
		conn, err := dial(context.Background(), "tcp", "fundsuggest.eastmoney.com:443")
		require.NoError(t, err)
		require.Same(t, client, conn)
		require.Equal(t, "fundsuggest.eastmoney.com:443", addresses[len(addresses)-1])
		client.Close()
		server.Close()
	}
}

func TestFundDNSRejectsInvalidAndPrivateAnswers(t *testing.T) {
	for _, body := range []string{
		`{"Status":3,"Answer":[{"type":1,"data":"103.220.165.216"}]}`,
		`{"Status":0,"Answer":[{"type":1,"data":"127.0.0.1"},{"type":1,"data":"192.168.1.1"},{"type":1,"data":"169.254.1.1"},{"type":1,"data":"0.0.0.0"}]}`,
		`{"Status":0,"Answer":[{"type":5,"data":"103.220.165.216"},{"type":1,"data":"invalid"}]}`,
		strings.Repeat(" ", 16*1024) + `{"Status":0,"Answer":[{"type":1,"data":"103.220.165.216"}]}`,
	} {
		_, _, err := readFundDNSAddresses(strings.NewReader(body))
		require.Error(t, err)
	}
	addresses, ttl, err := readFundDNSAddresses(strings.NewReader(`{"Status":0,"Answer":[{"type":5,"data":"fund.example.com","TTL":30},{"type":1,"data":"103.220.165.216","TTL":600}]}`))
	require.NoError(t, err)
	require.Equal(t, []string{"103.220.165.216"}, addresses)
	require.Equal(t, 30*time.Second, ttl)
}

func TestFundDNSCancellationDoesNotRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	dial := fundDNSDial(func(context.Context, string, string) (net.Conn, error) {
		return nil, ctx.Err()
	}, func(context.Context, string) ([]string, error) {
		t.Fatal("canceled request must not resolve again")
		return nil, nil
	})
	_, err := dial(ctx, "tcp", "fundsuggest.eastmoney.com:443")
	require.ErrorIs(t, err, context.Canceled)
}

type fundDNSTransport func(*http.Request) (*http.Response, error)

func (transport fundDNSTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return transport(req)
}

func TestFundDNSCacheExpiryAndFailedRefresh(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	requests, fail := 0, false
	resolver := &fundDNSResolver{now: func() time.Time { return now }, cache: make(map[string]cachedFundDNS)}
	resolver.client = &http.Client{Transport: fundDNSTransport(func(req *http.Request) (*http.Response, error) {
		requests++
		require.Equal(t, "https", req.URL.Scheme)
		require.Equal(t, "223.5.5.5", req.URL.Host)
		if fail {
			return nil, errors.New("temporary DNS failure")
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"Status":0,"Answer":[{"type":1,"data":"103.220.165.216","TTL":30}]}`)), Header: make(http.Header)}, nil
	})}
	ctx := context.Background()
	first, err := resolver.lookup(ctx, "fundsuggest.eastmoney.com")
	require.NoError(t, err)
	first[0] = "127.0.0.1" // A caller must not mutate the cached public answer.
	now = now.Add(29 * time.Second)
	second, err := resolver.lookup(ctx, "fundsuggest.eastmoney.com")
	require.NoError(t, err)
	require.Equal(t, []string{"103.220.165.216"}, second)
	require.Equal(t, 1, requests)
	now = now.Add(2 * time.Second)
	fail = true
	_, err = resolver.lookup(ctx, "fundsuggest.eastmoney.com")
	require.Error(t, err)
	fail = false
	_, err = resolver.lookup(ctx, "fundsuggest.eastmoney.com")
	require.NoError(t, err)
	require.Equal(t, 3, requests)
	_, err = resolver.lookup(ctx, "api.coinbase.com")
	require.Error(t, err)
	require.Equal(t, 3, requests)
}
