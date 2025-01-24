// internal/arbitrage/arbitrage.go
package arbitrage

import (
	"fmt"
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
			// log.Printf("Error fetching order book from exchange: %v", err)
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

	// Find the highest bid and lowest ask across all the different exchanges
	if len(bestBids) == 0 || len(bestAsks) == 0 {
		return exchange.Order{}, exchange.Order{}, fmt.Errorf("no bids or asks found")
	}

	highestBid = bestBids[0]
	lowestAsk = bestAsks[0]

	// the below approach sometimes leads to same exchange arbitrage opportunities
	// for _, bid := range bestBids {
	// 	if bid.Price > highestBid.Price {
	// 		highestBid = bid
	// 	}
	// }

	// for _, ask := range bestAsks {
	// 	if ask.Price < lowestAsk.Price {
	// 		lowestAsk = ask
	// 	}
	// }

	// Eliminating the same exchange arbitrage entirely
	foundValidPair := false

	for _, bid := range bestBids {
		for _, ask := range bestAsks {
			if bid.Exchange == ask.Exchange {
				continue
			}

			// Update highest bid if higher and from different exchange than current lowest ask
			if bid.Exchange != lowestAsk.Exchange && bid.Price > highestBid.Price {
				highestBid = bid
				foundValidPair = true
			}

			// Update lowest ask if lower and from different exchange than current highest bid
			if ask.Exchange != highestBid.Exchange && ask.Price < lowestAsk.Price {
				lowestAsk = ask
				foundValidPair = true
			}
		}
	}

	if !foundValidPair {
		// return exchange.Order{}, exchange.Order{}, fmt.Errorf("no valid cross-exchange opportunities found")
	}

	return lowestAsk, highestBid, nil
}

func CalculateNetProfitPercentage(ask, bid float64) float64 {
	tradeFee := 0.001 // Considering 0.1 per trade
	netProfit := (bid - ask) * (1 - tradeFee)
	netProfitPercentage := (netProfit / ask) * 100
	return netProfitPercentage
}
