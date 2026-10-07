package marketquotes

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"time"

	"golang.org/x/net/websocket"
)

type coinbaseProduct struct {
	ProductID       string `json:"product_id"`
	BaseCurrency    string `json:"base_currency_id"`
	QuoteCurrency   string `json:"quote_currency_id"`
	ProductType     string `json:"product_type"`
	Status          string `json:"status"`
	Disabled        bool   `json:"is_disabled"`
	TradingDisabled bool   `json:"trading_disabled"`
	BaseName        string `json:"base_name"`
	Change24h       string `json:"price_percentage_change_24h"`
}

func (s *Service) verifyProducts(ctx context.Context) error {
	query := url.Values{"limit": {"100"}}
	for _, candidate := range instruments {
		query.Add("product_ids", candidate.productID)
	}
	var response struct {
		Products []coinbaseProduct `json:"products"`
	}
	if err := s.getJSON(ctx, s.config.CoinbaseRESTURL+"/products?"+query.Encode(), nil, &response); err != nil {
		return err
	}
	verified := make(map[string]string)
	for _, product := range response.Products {
		for _, candidate := range instruments {
			if product.ProductID == candidate.productID && product.BaseCurrency == candidate.base && product.QuoteCurrency == "USD" && product.ProductType == "SPOT" && product.Status == "online" && !product.Disabled && !product.TradingDisabled {
				verified[product.ProductID] = candidate.id
			}
		}
	}
	s.mu.Lock()
	s.products, s.productsVerifiedAt = verified, s.config.Now()
	s.mu.Unlock()
	if len(verified) == 0 {
		return errors.New("no supported public Coinbase products")
	}
	return nil
}

func (s *Service) coinbaseLoop(ctx context.Context) {
	delay := s.config.ReconnectMin
	for ctx.Err() == nil {
		s.mu.RLock()
		verify := len(s.products) == 0 || s.config.Now().Sub(s.productsVerifiedAt) > 24*time.Hour
		s.mu.RUnlock()
		if verify {
			if err := s.verifyProducts(ctx); err != nil {
				if !waitContext(ctx, delay) {
					return
				}
				delay = nextBackoff(delay, s.config.ReconnectMax)
				continue
			}
		}
		started := time.Now()
		_ = s.stream(ctx)
		s.mu.Lock()
		s.connected = false
		s.mu.Unlock()
		if ctx.Err() != nil {
			return
		}
		s.refreshREST(ctx)
		if time.Since(started) > 30*time.Second {
			delay = s.config.ReconnectMin
		}
		if !waitContext(ctx, delay) {
			return
		}
		delay = nextBackoff(delay, s.config.ReconnectMax)
	}
}

func nextBackoff(current, maximum time.Duration) time.Duration {
	if current >= maximum/2 {
		return maximum
	}
	return current * 2
}

func (s *Service) restLoop(ctx context.Context) {
	ticker := time.NewTicker(s.config.RESTInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Refresh the product mapping even if a healthy socket stays up for
			// days. Delisted products stop receiving REST or streaming updates.
			s.mu.RLock()
			verify := s.config.Now().Sub(s.productsVerifiedAt) > 24*time.Hour
			s.mu.RUnlock()
			if verify {
				_ = s.verifyProducts(ctx)
			}
			s.refreshREST(ctx)
		}
	}
}

func (s *Service) refreshREST(ctx context.Context) {
	s.mu.Lock()
	now := s.config.Now()
	if len(s.products) == 0 || (!s.lastRESTAttempt.IsZero() && now.Sub(s.lastRESTAttempt) < s.config.RESTInterval) {
		s.mu.Unlock()
		return
	}
	s.lastRESTAttempt = now
	products := make(map[string]string)
	for product, id := range s.products {
		quote, exists := s.quotes[id]
		if !exists || s.quoteViewLocked(quote).State != StateLive {
			products[product] = id
		}
	}
	s.mu.Unlock()
	// Bounded workers keep a slow product from delaying the other four.
	keys := make([]string, 0, len(products))
	for product := range products {
		keys = append(keys, product)
	}
	runBounded(ctx, len(keys), 3, func(index int) {
		product := keys[index]
		id := products[product]
		if ctx.Err() != nil {
			return
		}
		var response struct {
			Trades []struct {
				ProductID string `json:"product_id"`
				Price     string `json:"price"`
				Time      string `json:"time"`
			} `json:"trades"`
		}
		address := s.config.CoinbaseRESTURL + "/products/" + url.PathEscape(product) + "/ticker?limit=1"
		if err := s.getJSON(ctx, address, nil, &response); err != nil || len(response.Trades) == 0 {
			return
		}
		trade := response.Trades[0]
		sourceTime, err := time.Parse(time.RFC3339Nano, trade.Time)
		if err != nil || trade.ProductID != product {
			return
		}
		s.putQuote(Quote{InstrumentID: id, Price: trade.Price, Currency: "USD", Source: SourceCoinbaseREST, ReceivedAt: s.config.Now().Unix(), State: StateDelayed}, sourceTime)
	})
}

type coinbaseMessage struct {
	Channel   string `json:"channel"`
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Sequence  *int64 `json:"sequence_num"`
	Events    []struct {
		Type             string `json:"type"`
		HeartbeatCounter *int64 `json:"heartbeat_counter"`
		Tickers          []struct {
			ProductID string `json:"product_id"`
			Price     string `json:"price"`
			Change24h string `json:"price_percent_chg_24_h"`
		} `json:"tickers"`
	} `json:"events"`
}

type streamState struct {
	lastSequence         int64
	lastHeartbeatCounter int64
	lastHeartbeat        time.Time
}

func (s *Service) stream(ctx context.Context) error {
	products := s.verifiedProducts()
	if len(products) == 0 {
		return errors.New("no verified Coinbase subscription")
	}
	config, err := websocket.NewConfig(s.config.CoinbaseWSURL, "https://www.coinbase.com")
	if err != nil {
		return err
	}
	config.Dialer = &net.Dialer{Timeout: s.config.ReadTimeout}
	config.Header.Set("User-Agent", "CYLedger/0.1 public-reference-prices")
	conn, err := s.dialMarketStream(ctx, config)
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.MaxPayloadBytes = 1024 * 1024
	stopped := make(chan struct{})
	defer close(stopped)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-stopped:
		}
	}()
	if err = conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return err
	}
	if err = websocket.JSON.Send(conn, map[string]interface{}{"type": "subscribe", "channel": "ticker_batch", "product_ids": products}); err != nil {
		return err
	}
	if err = websocket.JSON.Send(conn, map[string]interface{}{"type": "subscribe", "channel": "heartbeats"}); err != nil {
		return err
	}
	state := streamState{lastSequence: -1, lastHeartbeatCounter: -1, lastHeartbeat: time.Now()}
	for {
		// Only a real heartbeat extends this deadline. Busy ticker traffic does
		// not conceal the loss of the keepalive channel.
		if err = conn.SetReadDeadline(state.lastHeartbeat.Add(s.config.ReadTimeout)); err != nil {
			return err
		}
		var message coinbaseMessage
		if err = websocket.JSON.Receive(conn, &message); err != nil {
			return err
		}
		if err = s.consumeMessage(message, &state); err != nil {
			return err
		}
	}
}

func (s *Service) consumeMessage(message coinbaseMessage, state *streamState) error {
	if message.Type == "error" || message.Channel == "error" {
		return errors.New("Coinbase rejected subscription")
	}
	if message.Sequence == nil || *message.Sequence < 0 {
		return nil
	}
	// Sequences reset per connection. Drop duplicates and older envelopes,
	// including their heartbeats; a replay must not extend liveness.
	if *message.Sequence <= state.lastSequence {
		return nil
	}
	sourceTime, err := time.Parse(time.RFC3339Nano, message.Timestamp)
	if err != nil || sourceTime.After(s.config.Now().Add(2*time.Minute)) {
		return nil
	}
	state.lastSequence = *message.Sequence
	switch message.Channel {
	case "heartbeats":
		if s.config.Now().Sub(sourceTime) > s.config.ReadTimeout {
			return nil
		}
		for _, event := range message.Events {
			if event.HeartbeatCounter == nil || *event.HeartbeatCounter <= state.lastHeartbeatCounter {
				continue
			}
			state.lastHeartbeatCounter = *event.HeartbeatCounter
			state.lastHeartbeat = time.Now()
			s.mu.Lock()
			s.connected, s.heartbeatAt = true, s.config.Now()
			s.mu.Unlock()
		}
	case "ticker_batch":
		// A very old snapshot cannot become live merely because a socket is up.
		quality := StateLive
		if s.config.Now().Sub(sourceTime) > s.config.StaleAfter {
			quality = StateStale
		}
		for _, event := range message.Events {
			if event.Type != "snapshot" && event.Type != "update" {
				continue
			}
			for _, ticker := range event.Tickers {
				s.mu.RLock()
				id, exists := s.products[ticker.ProductID]
				s.mu.RUnlock()
				if !exists {
					continue
				}
				s.putQuote(Quote{InstrumentID: id, Price: ticker.Price, Currency: "USD", Source: SourceCoinbaseWS, ReceivedAt: s.config.Now().Unix(), State: quality, ChangePercent: signedPercent(ticker.Change24h), ChangePeriod: "24h"}, sourceTime)
			}
		}
	case "subscriptions":
		// An acknowledgement is not a price or a heartbeat.
	default:
		// Coinbase may introduce channels; unrelated messages are harmless.
	}
	return nil
}

// Probe checks real provider access without starting persistent background work.
// It is only used by the explicitly enabled live integration test.
func (s *Service) Probe(ctx context.Context) error {
	if err := s.verifyProducts(ctx); err != nil {
		return fmt.Errorf("Coinbase products: %w", err)
	}
	s.refreshREST(ctx)
	for _, id := range []string{"crypto:bitcoin", "crypto:ethereum"} {
		quote, ok := s.Get(id)
		if !ok || quote.State != StateDelayed {
			return fmt.Errorf("no recent REST quote for %s", id)
		}
	}
	if err := s.refreshFX(ctx); err != nil {
		return fmt.Errorf("ECB reference rate: %w", err)
	}
	return nil
}
