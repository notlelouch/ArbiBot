package main

import (
	"context"
	"log"
	"math"
	"notlelouch/ArbiBot/internal/arbitrage"
	"notlelouch/ArbiBot/internal/exchange"
	"notlelouch/ArbiBot/internal/exchange/hyperliquid"
	"notlelouch/ArbiBot/internal/exchange/kucoin"
	"notlelouch/ArbiBot/internal/ui"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func runArbitrageMonitoring(ctx context.Context, ad *ui.ArbitrageDashboard, coins []string) {
	// Get public token for KuCoin
	tokenResp, err := kucoin.GetToken("", "", "", false)
	if err != nil {
		log.Fatalf("Failed to get public token: %v", err)
	}

	// Initialize exchange clients
	hyperliquidClient := hyperliquid.NewHyperliquidWS(true)
	kucoinClient := kucoin.NewKuCoinWS(tokenResp)

	// Connect to exchanges
	if err := hyperliquidClient.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect to Hyperliquid: %v", err)
	}
	if err := kucoinClient.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect to KuCoin: %v", err)
	}

	// Wait for order books to be populated
	log.Println("Waiting for order book updates...")
	time.Sleep(2000 * time.Millisecond)

	// Start updating arbitrage data for each coin
	var wg sync.WaitGroup
	for _, coin := range coins {
		wg.Add(1)
		go func(symbol string) {
			defer wg.Done()

			// Subscribe to order books
			if err := hyperliquidClient.SubscribeToOrderBook(symbol); err != nil {
				log.Printf("Failed to subscribe to Hyperliquid order book for %s: %v\n", symbol, err)
				return
			}
			if err := kucoinClient.SubscribeToOrderBook(symbol); err != nil {
				log.Printf("Failed to subscribe to KuCoin order book for %s: %v\n", symbol, err)
				return
			}

			// Run arbitrage logic continuously in a goroutine
			ticker := time.NewTicker(400 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					exchanges := []exchange.Exchange{hyperliquidClient, kucoinClient}
					lowestAsk, highestBid, err := arbitrage.FindBestPrices(exchanges, symbol)
					if err != nil {
						log.Printf("Error finding best prices for %s: %v\n", symbol, err)
						continue
					}

					// Calculate arbitrage profit
					profit := 0.0
					spread := 0.0
					if highestBid.Price > lowestAsk.Price {
						profit = arbitrage.CalculateNetProfitPercentage(lowestAsk.Price, highestBid.Price)
						spread = math.Abs(highestBid.Price-lowestAsk.Price) / lowestAsk.Price * 100

						// Send update to dashboard
						ad.SendCoinData(ui.CoinData{
							Symbol:       symbol,
							BuyExchange:  lowestAsk.Exchange,
							BuyPrice:     lowestAsk.Price,
							SellExchange: highestBid.Exchange,
							SellPrice:    highestBid.Price,
							Profit:       profit,
							Spread:       spread,
							Timestamp:    time.Now(),
						},
						)
					}
				case <-ctx.Done():
					return
				}
			}
		}(coin)
	}

	wg.Wait()
}

func main() {
	// Define the list of coins to monitor
	coins := []string{
		"SOL", "ATOM", "BTC", "APT", "ETH",
	}

	// Create a context that can be canceled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutting down gracefully...")
		cancel()
	}()

	// Create Arbitrage Dashboard
	ad := ui.NewArbitrageDashboard(coins)

	// Initialize widgets
	if err := ad.InitWidgets(); err != nil {
		log.Fatalf("Failed to initialize widgets: %v", err)
	}

	// Start update listener
	ad.StartUpdateListener(ctx)

	// Run arbitrage monitoring in parallel with terminal dashboard
	go runArbitrageMonitoring(ctx, ad, coins)

	// Run the terminal dashboard
	if err := ui.RunDashboard(ctx, ad); err != nil {
		log.Fatalf("Failed to run dashboard: %v", err)
	}

	// Keep the program running until the context is canceled
	<-ctx.Done()
	log.Println("Program exited gracefully.")
}
