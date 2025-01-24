// internal/exchange/hyperliquid/hyperliquid.go
package hyperliquid

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"notlelouch/ArbiBot/internal/exchange"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type HyperliquidWS struct {
	handlers   map[string]func([]byte)
	conn       *websocket.Conn
	orderBooks map[string]*struct {
		timeStamp time.Time
		book      exchange.OrderBook
	}
	url string
	mu  sync.RWMutex
}

func NewHyperliquidWS(mainnet bool) *HyperliquidWS {
	url := "wss://api.hyperliquid-testnet.xyz/ws"
	if mainnet {
		url = "wss://api.hyperliquid.xyz/ws"
	}
	return &HyperliquidWS{
		url:      url,
		handlers: make(map[string]func([]byte)),
		orderBooks: make(map[string]*struct {
			timeStamp time.Time
			book      exchange.OrderBook
		}),
	}
}

func (h *HyperliquidWS) Connect(ctx context.Context) error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 15 * time.Second,
	}
	conn, _, err := dialer.DialContext(ctx, h.url, nil)
	if err != nil {
		return fmt.Errorf("websocket connection failed: %w", err)
	}
	h.conn = conn
	go h.handleMessages(ctx)
	return nil
}

func (h *HyperliquidWS) SubscribeToOrderBook(coin string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	subscription := SubscriptionMessage{
		Method: "subscribe",
		Subscription: Subscription{
			Type: "l2Book",
			Coin: coin,
		},
	}
	return h.conn.WriteJSON(subscription)
}

func (h *HyperliquidWS) GetOrderBook(coin string) (exchange.OrderBook, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	orderBookData, exists := h.orderBooks[coin]
	if !exists {
		return exchange.OrderBook{}, fmt.Errorf("no order book for %s", coin)
	}

	// Check if data is stale (older than 5 seconds)
	if time.Since(orderBookData.timeStamp) > 2000*time.Millisecond {
		return exchange.OrderBook{}, fmt.Errorf("stale order book for %s", coin)
	}
	return orderBookData.book, nil
}

func (h *HyperliquidWS) handleMessages(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			_, message, err := h.conn.ReadMessage()
			if err != nil {
				log.Printf("read error: %v", err)
				return
			}
			// log.Printf("Hyperliquid WebSocket message: %s", message)
			var response WsResponse
			if err := json.Unmarshal(message, &response); err != nil {
				log.Printf("unmarshal error: %v", err)
				continue
			}
			switch response.Channel {
			case "l2Book":
				var orderbook WsBook
				if err := json.Unmarshal(response.Data, &orderbook); err != nil {
					log.Printf("l2Book unmarshal error: %v", err)
					continue
				}

				h.mu.Lock()
				// Initialize or clear the orderbook for this coin
				coin := orderbook.Coin // Assuming this field exists in WsBook
				if _, exists := h.orderBooks[coin]; !exists {
					h.orderBooks[coin] = &struct {
						timeStamp time.Time
						book      exchange.OrderBook
					}{
						book:      exchange.OrderBook{},
						timeStamp: time.Now(),
					}
				}

				newBook := exchange.OrderBook{
					Bids: make([]exchange.Order, 0, len(orderbook.Levels[1])),
					Asks: make([]exchange.Order, 0, len(orderbook.Levels[0])),
				}

				// Process bids and asks
				for _, level := range orderbook.Levels[1] { // Bids
					price, _ := strconv.ParseFloat(level.Px, 64)
					amount, _ := strconv.ParseFloat(level.Sz, 64)
					newBook.Bids = append(newBook.Bids, exchange.Order{
						Price:    price,
						Amount:   amount,
						Exchange: "Hyperliquid",
					})
				}
				for _, level := range orderbook.Levels[0] { // Asks
					price, _ := strconv.ParseFloat(level.Px, 64)
					amount, _ := strconv.ParseFloat(level.Sz, 64)
					newBook.Asks = append(newBook.Asks, exchange.Order{
						Price:    price,
						Amount:   amount,
						Exchange: "Hyperliquid",
					})
				}

				h.orderBooks[coin].book = newBook
				h.orderBooks[coin].timeStamp = time.Now()
				h.mu.Unlock()
			default:
				log.Printf("Unhandled channel: %s", response.Channel)
			}
		}
	}
}
