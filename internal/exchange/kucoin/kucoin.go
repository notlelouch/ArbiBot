// internal/exchange/kucoin/kucoin.go
package kucoin

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

type KuCoinWS struct {
	conn         *websocket.Conn
	endpoint     string
	token        string
	orderBook    exchange.OrderBook
	mu           sync.Mutex // Mutex to protect order book
	pingInterval time.Duration
}

func NewKuCoinWS(tokenResp *TokenResponse) *KuCoinWS {
	return &KuCoinWS{
		endpoint:     tokenResp.Data.InstanceServers[0].Endpoint,
		token:        tokenResp.Data.Token,
		pingInterval: time.Duration(tokenResp.Data.InstanceServers[0].PingInterval) * time.Millisecond,
		orderBook:    exchange.OrderBook{Bids: []exchange.Order{}, Asks: []exchange.Order{}},
	}
}

func (k *KuCoinWS) Connect(ctx context.Context) error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 15 * time.Second,
	}
	conn, _, err := dialer.DialContext(ctx, k.endpoint+"?token="+k.token, nil)
	if err != nil {
		return fmt.Errorf("WebSocket connection failed: %w", err)
	}
	k.conn = conn
	log.Println("Successfully connected to KuCoin WebSocket") // Add logging
	// Start ping loop
	go k.pingLoop(ctx)

	// Handle messages
	go k.handleMessages(ctx)

	return nil
}

func (k *KuCoinWS) SubscribeToOrderBook(coin string) error {
	topic := "/market/level2:" + coin + "-USDT"
	// return k.Subscribe(topic, false)
	err := k.Subscribe(topic, false)
	if err != nil {
		return fmt.Errorf("failed to subscribe to KuCoin order book: %w", err)
	}
	log.Printf("Successfully subscribed to KuCoin order book for %s", coin) // Add logging
	return nil
}

func (k *KuCoinWS) GetOrderBook(coin string) (exchange.OrderBook, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.orderBook, nil
}

func (k *KuCoinWS) pingLoop(ctx context.Context) {
	ticker := time.NewTicker(k.pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := k.conn.WriteJSON(map[string]string{"id": "ping", "type": "ping"}); err != nil {
				log.Printf("ping error: %v", err)
				return
			}
		}
	}
}

func (k *KuCoinWS) handleMessages(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			_, message, err := k.conn.ReadMessage()
			if err != nil {
				log.Printf("KuCoin WebSocket read error: %v", err)
				return
			}
			// log.Printf("KuCoin WebSocket message: %s", message)

			var msg struct {
				Topic string `json:"topic"`
				Data  struct {
					Changes struct {
						Asks [][]string `json:"asks"`
						Bids [][]string `json:"bids"`
					} `json:"changes"`
				} `json:"data"`
			}
			if err := json.Unmarshal(message, &msg); err != nil {
				log.Printf("KuCoin WebSocket unmarshal error: %v", err)
				continue
			}

			// Log the parsed message
			// log.Printf("KuCoin Order Book Update - Asks: %v, Bids: %v", msg.Data.Changes.Asks, msg.Data.Changes.Bids)
			// Update local order book

			k.mu.Lock()

			k.orderBook.Asks = []exchange.Order{} // Clear existing asks
			k.orderBook.Bids = []exchange.Order{} // Clear existing bids

			for _, ask := range msg.Data.Changes.Asks {
				price, _ := strconv.ParseFloat(ask[0], 64)
				size, _ := strconv.ParseFloat(ask[1], 64)
				k.orderBook.Asks = append(k.orderBook.Asks, exchange.Order{
					Price:    price,
					Amount:   size,
					Exchange: "KuCoin",
				})
			}
			for _, bid := range msg.Data.Changes.Bids {
				price, _ := strconv.ParseFloat(bid[0], 64)
				size, _ := strconv.ParseFloat(bid[1], 64)
				k.orderBook.Bids = append(k.orderBook.Asks, exchange.Order{
					Price:    price,
					Amount:   size,
					Exchange: "KuCoin",
				})
			}
			k.mu.Unlock()
		}
	}
}

func (k *KuCoinWS) Subscribe(topic string, privateChannel bool) error {
	subscription := map[string]interface{}{
		"id":             "1",
		"type":           "subscribe",
		"topic":          topic,
		"privateChannel": privateChannel,
		"response":       true,
	}
	return k.conn.WriteJSON(subscription)
}
