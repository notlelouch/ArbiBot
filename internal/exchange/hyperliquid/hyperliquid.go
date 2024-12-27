// internal/exchange/hyperliquid/hyperliquid.go
package hyperliquid

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"notlelouch/ArbiBot/internal/exchange"
	"time"

	"github.com/gorilla/websocket"
)

type HyperliquidWS struct {
	handlers map[string]func([]byte)
	conn     *websocket.Conn
	url      string
}

func NewHyperliquidWS(mainnet bool) *HyperliquidWS {
	url := "wss://api.hyperliquid-testnet.xyz/ws"
	if mainnet {
		url = "wss://api.hyperliquid.xyz/ws"
	}
	return &HyperliquidWS{
		url:      url,
		handlers: make(map[string]func([]byte)),
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
	// Implement logic to fetch and return the order book
	// // Fetch the order book in Hyperliquid's format (WsBook)
	// var wsBook WsBook
	// // Convert WsBook to the generic OrderBook
	// var orderBook exchange.OrderBook
	// for _, level := range wsBook.Levels[1] { // Bids
	// 	price, _ := strconv.ParseFloat(level.Px, 64)
	// 	amount, _ := strconv.ParseFloat(level.Sz, 64)
	// 	orderBook.Bids = append(orderBook.Bids, exchange.Order{Price: price, Amount: amount})
	// }
	// for _, level := range wsBook.Levels[0] { // Asks
	// 	price, _ := strconv.ParseFloat(level.Px, 64)
	// 	amount, _ := strconv.ParseFloat(level.Sz, 64)
	// 	orderBook.Asks = append(orderBook.Asks, exchange.Order{Price: price, Amount: amount})
	// }
	// return orderBook, nil
	return exchange.OrderBook{}, nil
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
				log.Printf("Order Book: %+v", orderbook)
			default:
				log.Printf("Unhandled channel: %s", response.Channel)
			}
		}
	}
}
