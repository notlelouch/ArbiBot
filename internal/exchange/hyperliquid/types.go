package hyperliquid

import "encoding/json"

// WsResponse represents the WebSocket response
type WsResponse struct {
	Channel string          `json:"channel"`
	Data    json.RawMessage `json:"data"`
}

// SubscriptionMessage represents the WebSocket subscription request
type SubscriptionMessage struct {
	Method       string       `json:"method"`
	Subscription Subscription `json:"subscription"`
}

type Subscription struct {
	Type string `json:"type"`
	Coin string `json:"coin,omitempty"`
}

type WsTrade struct {
	Coin  string   `json:"coin"`
	Side  string   `json:"side"`
	Px    string   `json:"px"`
	Sz    string   `json:"sz"`
	Hash  string   `json:"hash"`
	Users []string `json:"users"`
	Time  int64    `json:"time"`
	Tid   int64    `json:"tid"`
}

type WsLevel struct {
	Px string `json:"px"` // price
	Sz string `json:"sz"` // size
	N  int    `json:"n"`  // number of orders
}

type WsBook struct {
	Coin   string       `json:"coin"`
	Levels [2][]WsLevel `json:"levels"` // 0: asks, 1: bids
	Time   int64        `json:"time"`
}
