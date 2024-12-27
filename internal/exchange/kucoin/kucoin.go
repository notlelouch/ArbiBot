// internal/exchange/kucoin/kucoin.go
package kucoin

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

type KuCoinWS struct {
	conn         *websocket.Conn
	endpoint     string
	token        string
	pingInterval time.Duration
}

func NewKuCoinWS(tokenResp *TokenResponse) *KuCoinWS {
	return &KuCoinWS{
		endpoint:     tokenResp.Data.InstanceServers[0].Endpoint,
		token:        tokenResp.Data.Token,
		pingInterval: time.Duration(tokenResp.Data.InstanceServers[0].PingInterval) * time.Millisecond,
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

	// Start ping loop
	go k.pingLoop(ctx)

	// Handle messages
	go k.handleMessages(ctx)

	return nil
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
				log.Printf("read error: %v", err)
				return
			}
			log.Printf("Received message: %s", message)
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
