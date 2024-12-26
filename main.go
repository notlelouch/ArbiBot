package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

// WsResponse represents the WebSocket response
type WsResponse struct {
	Channel string          `json:"channel"`
	Data    json.RawMessage `json:"data"`
}

type HyperliquidWS struct {
	conn     *websocket.Conn
	url      string
	handlers map[string]func([]byte)
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

	// Start message handler
	go h.handleMessages(ctx)

	return nil
}

func (h *HyperliquidWS) SubscribeToTrades(coin string) error {
	subscription := SubscriptionMessage{
		Method: "subscribe",
		Subscription: Subscription{
			Type: "trades",
			Coin: coin,
		},
	}

	return h.conn.WriteJSON(subscription)
}

func (h *HyperliquidWS) SubscribeToL2Book(coin string) error {
	subscription := SubscriptionMessage{
		Method: "subscribe",
		Subscription: Subscription{
			Type: "l2Book",
			Coin: coin,
		},
	}

	return h.conn.WriteJSON(subscription)
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

			// Handle different response types
			switch response.Channel {
			case "trades":
				var trades []WsTrade
				if err := json.Unmarshal(response.Data, &trades); err != nil {
					log.Printf("trades unmarshal error: %v", err)
					continue
				}
				for _, trade := range trades {
					log.Printf("Trade: %+v", trade)
				}
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

func main() {
	ctx := context.Background()

	// Create new client (using testnet for this example)
	client := NewHyperliquidWS(true)

	// Connect to WebSocket
	if err := client.Connect(ctx); err != nil {
		log.Fatal("connection error:", err)
	}

	// // Subscribe to SOL trades
	// if err := client.SubscribeToTrades("SOL"); err != nil {
	// 	log.Fatal("subscription error:", err)
	// }

	// Subscribe to SOL l2 order book
	if err := client.SubscribeToL2Book("SOL"); err != nil {
		log.Fatal("subscription error:", err)
	}

	// Keep the connection alive
	select {}
}
