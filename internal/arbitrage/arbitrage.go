// internal/arbitrage/arbitrage.go
package arbitrage

import (
	"fmt"
	"log"
	"notlelouch/ArbiBot/internal/exchange"
)

// FindBestPrices finds the lowest ask and highest bid across multiple exchanges.
func FindBestPrices(exchanges []exchange.Exchange, coin string) (lowestAsk, highestBid exchange.Order, err error) {
	var bestBids []exchange.Order
	var bestAsks []exchange.Order

	// Fetch order books from all exchanges
	for _, ex := range exchanges {
		orderBook, err := ex.GetOrderBook(coin)
		// log.Printf("orderBook Bids: %v", orderBook.Bids) // Fixed logging
		if err != nil {
			log.Printf("Error fetching order book from exchange: %v", err)
			continue
		}

		// Extract best bid and ask
		if len(orderBook.Bids) > 0 {
			bestBids = append(bestBids, orderBook.Bids[0]) // Bids are sorted highest to lowest
		}
		if len(orderBook.Asks) > 0 {
			bestAsks = append(bestAsks, orderBook.Asks[0]) // Asks are sorted lowest to highest
		}
	}

	// Find the highest bid and lowest ask across all exchanges
	if len(bestBids) == 0 || len(bestAsks) == 0 {
		return exchange.Order{}, exchange.Order{}, fmt.Errorf("no bids or asks found")
	}

	highestBid = bestBids[0]
	for _, bid := range bestBids {
		if bid.Price > highestBid.Price {
			highestBid = bid
		}
	}

	lowestAsk = bestAsks[0]
	for _, ask := range bestAsks {
		if ask.Price < lowestAsk.Price {
			lowestAsk = ask
		}
	}

	// log.Printf("exiting the FindBestPrices function")
	return lowestAsk, highestBid, nil
}
