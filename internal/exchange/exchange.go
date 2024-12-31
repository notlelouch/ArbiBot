// internal/exchange/exchange.go
package exchange

import "context"

type Exchange interface {
	Connect(ctx context.Context) error
	SubscribeToOrderBook(coin string) error
	GetOrderBook(coin string) (OrderBook, error)
}

type OrderBook struct {
	Bids []Order
	Asks []Order
}

type Order struct {
	Exchange string
	Price    float64
	Amount   float64
}
