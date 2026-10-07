package marketquotes

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type fundTestTransport func(*http.Request) (*http.Response, error)

func (f fundTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestFundTransportHedgesStalledRouteAndPreservesWinningBody(t *testing.T) {
	cancelled := make(chan struct{})
	primary := fundTestTransport(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		close(cancelled)
		return nil, r.Context().Err()
	})
	var winning context.Context
	fallback := fundTestTransport(func(r *http.Request) (*http.Response, error) {
		winning = r.Context()
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("verified response")), Header: make(http.Header)}, nil
	})
	req, _ := http.NewRequest("GET", "https://fundsuggest.eastmoney.com/", nil)
	transport := fundTransport{primary: primary, fallback: fallback, delay: time.Millisecond}
	response, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	if winning.Err() != nil {
		t.Fatal("cancelled winning response before body read")
	}
	body, err := io.ReadAll(response.Body)
	if err != nil || string(body) != "verified response" {
		t.Fatal("lost winning body")
	}
	response.Body.Close()
	if winning.Err() == nil {
		t.Fatal("winning context leaked")
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("stalled route leaked")
	}
}
