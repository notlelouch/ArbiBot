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
	handlers  map[string]func([]byte)
	conn      *websocket.Conn
	url       string
	orderBook exchange.OrderBook
	mu        sync.Mutex // Mutex to protect order book
}

func NewHyperliquidWS(mainnet bool) *HyperliquidWS {
	url := "wss://api.hyperliquid-testnet.xyz/ws"
	if mainnet {
		url = "wss://api.hyperliquid.xyz/ws"
	}
	return &HyperliquidWS{
		url:       url,
		handlers:  make(map[string]func([]byte)),
		orderBook: exchange.OrderBook{Bids: []exchange.Order{}, Asks: []exchange.Order{}},
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
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.orderBook, nil
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
				// log.Printf("Hyperliquid Order Book Update: %+v", orderbook)

				// Update local order book
				h.mu.Lock()
				h.orderBook.Bids = []exchange.Order{}
				h.orderBook.Asks = []exchange.Order{}
				for _, level := range orderbook.Levels[1] { // Bids
					price, _ := strconv.ParseFloat(level.Px, 64)
					amount, _ := strconv.ParseFloat(level.Sz, 64)
					// h.orderBook.Bids = append(h.orderBook.Bids, exchange.Order{Price: price, Amount: amount})
					h.orderBook.Asks = append(h.orderBook.Asks, exchange.Order{
						Price:    price,
						Amount:   amount,
						Exchange: "Hyperliquid",
					})
				}
				for _, level := range orderbook.Levels[0] { // Asks
					price, _ := strconv.ParseFloat(level.Px, 64)
					amount, _ := strconv.ParseFloat(level.Sz, 64)
					// h.orderBook.Asks = append(h.orderBook.Asks, exchange.Order{Price: price, Amount: amount})
					h.orderBook.Bids = append(h.orderBook.Bids, exchange.Order{
						Price:    price,
						Amount:   amount,
						Exchange: "Hyperliquid",
					})
				}
				h.mu.Unlock()
			default:
				log.Printf("Unhandled channel: %s", response.Channel)
			}
		}
	}
}
