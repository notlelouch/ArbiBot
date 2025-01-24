package main

import (
	"context"
	"fmt"
	"log"
	"notlelouch/ArbiBot/internal/arbitrage"
	"notlelouch/ArbiBot/internal/exchange"
	"notlelouch/ArbiBot/internal/exchange/hyperliquid"
	"notlelouch/ArbiBot/internal/exchange/kucoin"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/mum4k/termdash"
	"github.com/mum4k/termdash/cell"
	"github.com/mum4k/termdash/container"
	"github.com/mum4k/termdash/container/grid"
	"github.com/mum4k/termdash/linestyle"
	"github.com/mum4k/termdash/terminal/tcell"
	"github.com/mum4k/termdash/terminal/terminalapi"
	"github.com/mum4k/termdash/widgets/barchart"
	"github.com/mum4k/termdash/widgets/text"
)

const (
	redrawInterval = 250 * time.Millisecond
)

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

	// Initialize terminal
	t, err := tcell.New(tcell.ColorMode(terminalapi.ColorMode256))
	if err != nil {
		log.Fatalf("Failed to initialize terminal: %v", err)
	}
	defer t.Close()

	// Create widgets for each coin
	coinWidgets := make(map[string]*text.Text)
	for _, coin := range coins {
		widget, err := text.New(text.RollContent(), text.WrapAtWords())
		if err != nil {
			log.Fatalf("Failed to create text widget for %s: %v", coin, err)
		}
		coinWidgets[coin] = widget
	}

	// Create bar chart for profits
	barChart, err := barchart.New(
		barchart.BarColors([]cell.Color{
			cell.ColorGreen,
			cell.ColorBlue,
			cell.ColorCyan,
			cell.ColorMagenta,
			cell.ColorYellow,
		}),
		barchart.ShowValues(),
		barchart.Labels(coins),
	)
	if err != nil {
		log.Fatalf("Failed to create bar chart: %v", err)
	}

	// Sync mechanism for updating bar chart
	profitsMutex := &sync.Mutex{}
	profits := make(map[string]float64)

	// Create a grid layout with one cell per coin and a bar chart
	builder := grid.New()
	builder.Add(
		grid.RowHeightPerc(25,
			grid.ColWidthPerc(20,
				grid.Widget(coinWidgets["SOL"],
					container.Border(linestyle.Light),
					container.BorderTitle(" SOL Arbitrage "),
				),
			),
			grid.ColWidthPerc(20,
				grid.Widget(coinWidgets["ATOM"],
					container.Border(linestyle.Light),
					container.BorderTitle(" ATOM Arbitrage "),
				),
			),
			grid.ColWidthPerc(20,
				grid.Widget(coinWidgets["BTC"],
					container.Border(linestyle.Light),
					container.BorderTitle(" BTC Arbitrage "),
				),
			),
			grid.ColWidthPerc(20,
				grid.Widget(coinWidgets["APT"],
					container.Border(linestyle.Light),
					container.BorderTitle(" APT Arbitrage "),
				),
			),
			grid.ColWidthPerc(20,
				grid.Widget(coinWidgets["ETH"],
					container.Border(linestyle.Light),
					container.BorderTitle(" ETH Arbitrage "),
				),
			),
		),
		grid.RowHeightPerc(75,
			grid.Widget(barChart,
				container.Border(linestyle.Light),
				container.BorderTitle(" Arbitrage Profits "),
			),
		),
	)

	// Build the grid layout
	gridOpts, err := builder.Build()
	if err != nil {
		log.Fatalf("Failed to build grid layout: %v", err)
	}

	// Create the root container with the grid layout
	c, err := container.New(t, gridOpts...)
	if err != nil {
		log.Fatalf("Failed to create root container: %v", err)
	}

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
			ticker := time.NewTicker(1 * time.Second) // Update every second
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
					if highestBid.Price > lowestAsk.Price {
						profit = arbitrage.CalculateNetProfitPercentage(lowestAsk.Price, highestBid.Price)

						// Update coin-specific widget
						coinWidgets[symbol].Reset()
						coinWidgets[symbol].Write(fmt.Sprintf(`
Buy Exchange: %s
Buy Price:    $%.8f
Sell Exchange: %s
Sell Price:    $%.8f
Profit:        %.4f%%
Time:          %s
`,
							lowestAsk.Exchange, lowestAsk.Price,
							highestBid.Exchange, highestBid.Price,
							profit,
							time.Now().Format("15:04:05"),
						))

						// Update profits for bar chart
						profitsMutex.Lock()
						profits[symbol] = profit

						// Prepare bar chart data
						barData := make([]int, len(coins))
						for i, coin := range coins {
							barData[i] = int(profits[coin] * 20000) // Scale for visibility
						}

						barChart.Values(barData, 1000) // Set the scale to 2000 for visibility
						profitsMutex.Unlock()
					}
				case <-ctx.Done():
					return
				}
			}
		}(coin)
	}

	// Run the terminal dashboard
	if err := termdash.Run(ctx, t, c, termdash.RedrawInterval(redrawInterval)); err != nil {
		log.Fatalf("Failed to run termdash: %v", err)
	}

	// Keep the program running until the context is canceled
	<-ctx.Done()
	log.Println("Program exited gracefully.")
}
