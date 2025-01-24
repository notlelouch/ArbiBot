// internal/exchange/kucoin/kucoin.go
package kucoin

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"notlelouch/ArbiBot/internal/exchange"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type KuCoinWS struct {
	conn       *websocket.Conn
	orderBooks map[string]*struct {
		timeStamp time.Time
		book      exchange.OrderBook
	}

	endpoint     string
	token        string
	pingInterval time.Duration
	mu           sync.RWMutex
}

// Level2Depth5Message represents the structure for level2depth5 messages
type Level2Depth5Message struct {
	Type    string `json:"type"`
	Topic   string `json:"topic"`
	Subject string `json:"subject"`
	Data    struct {
		Asks      [][]string `json:"asks"`      // [price, size]
		Bids      [][]string `json:"bids"`      // [price, size]
		Timestamp int64      `json:"timestamp"` // milliseconds
	} `json:"data"`
}

func NewKuCoinWS(tokenResp *TokenResponse) *KuCoinWS {
	return &KuCoinWS{
		endpoint:     tokenResp.Data.InstanceServers[0].Endpoint,
		token:        tokenResp.Data.Token,
		pingInterval: time.Duration(tokenResp.Data.InstanceServers[0].PingInterval) * time.Millisecond,
		orderBooks: make(map[string]*struct {
			timeStamp time.Time
			book      exchange.OrderBook
		}),
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
	log.Println("Successfully connected to KuCoin WebSocket")

	// Start ping loop
	go k.pingLoop(ctx)

	// Handle messages
	go k.handleMessages(ctx)

	return nil
}

func (k *KuCoinWS) SubscribeToOrderBook(coin string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	// Changed to use level2Depth5 topic
	topic := fmt.Sprintf("/spotMarket/level2Depth5:%s-USDT", coin)
	err := k.Subscribe(topic, false)
	if err != nil {
		return fmt.Errorf("failed to subscribe to KuCoin order book: %w", err)
	}
	log.Printf("Successfully subscribed to KuCoin level2Depth5 for %s", coin)
	return nil
}

func (k *KuCoinWS) GetOrderBook(coin string) (exchange.OrderBook, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	orderBookData, exists := k.orderBooks[coin]
	if !exists {
		return exchange.OrderBook{}, fmt.Errorf("no order book for %s", coin)
	}

	// Check if data is stale (older than 5 seconds)
	if time.Since(orderBookData.timeStamp) > 2000*time.Millisecond {
		return exchange.OrderBook{}, fmt.Errorf("stale order book for %s", coin)
	}
	return orderBookData.book, nil
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
				// Consider implementing reconnection logic here
				return
			}

			// Try to parse as Level2Depth5Message
			var l2Msg Level2Depth5Message
			if err := json.Unmarshal(message, &l2Msg); err != nil {
				// If it's not a level2depth5 message, it might be another type of message
				continue
			}
			// log.Printf("KuCoin WebSocket message: %s", message)

			// Only process if it's a level2 message
			if l2Msg.Subject == "level2" {
				k.updateOrderBook(&l2Msg)
			}
		}
	}
}

func (k *KuCoinWS) updateOrderBook(msg *Level2Depth5Message) {
	k.mu.Lock()
	defer k.mu.Unlock()

	// Extract coin from topic
	parts := strings.Split(msg.Topic, ":")
	if len(parts) != 2 {
		return
	}
	coin := strings.TrimSuffix(parts[1], "-USDT")

	// Initialize the order book entry if it doesn't exist
	if _, exists := k.orderBooks[coin]; !exists {
		k.orderBooks[coin] = &struct {
			timeStamp time.Time
			book      exchange.OrderBook
		}{
			book:      exchange.OrderBook{},
			timeStamp: time.Now(),
		}
	}
	// Create new slices for bids and asks
	newBids := make([]exchange.Order, 0, len(msg.Data.Bids))
	newAsks := make([]exchange.Order, 0, len(msg.Data.Asks))

	// Process bids
	for _, bid := range msg.Data.Bids {
		if len(bid) < 2 {
			continue
		}
		price, err := strconv.ParseFloat(bid[0], 64)
		if err != nil {
			continue
		}
		size, err := strconv.ParseFloat(bid[1], 64)
		if err != nil {
			continue
		}
		newBids = append(newBids, exchange.Order{
			Price:    price,
			Amount:   size,
			Exchange: "KuCoin",
		})
	}

	// Process asks
	for _, ask := range msg.Data.Asks {
		if len(ask) < 2 {
			continue
		}
		price, err := strconv.ParseFloat(ask[0], 64)
		if err != nil {
			continue
		}
		size, err := strconv.ParseFloat(ask[1], 64)
		if err != nil {
			continue
		}
		newAsks = append(newAsks, exchange.Order{
			Price:    price,
			Amount:   size,
			Exchange: "KuCoin",
		})
	}

	// Update the specific coin's order book
	// Update with new timestamp
	k.orderBooks[coin].book = exchange.OrderBook{
		Bids: newBids,
		Asks: newAsks,
	}
	k.orderBooks[coin].timeStamp = time.Now()
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
