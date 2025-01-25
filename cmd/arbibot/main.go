package main

import (
	"context"
	"fmt"
	"log"
	"math"
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
	"github.com/mum4k/termdash/widgets/linechart"
	"github.com/mum4k/termdash/widgets/text"
)

const (
	redrawInterval = 250 * time.Millisecond
	maxHistorySize = 50
)

type CoinData struct {
	Timestamp    time.Time
	Symbol       string
	BuyExchange  string
	SellExchange string
	BuyPrice     float64
	SellPrice    float64
	Profit       float64
	Spread       float64
}

type ArbitrageDashboard struct {
	coinWidgets    map[string]*text.Text
	barChart       *barchart.BarChart
	lineChart      *linechart.LineChart
	updateChan     chan CoinData
	closeChan      chan struct{}
	spreadsHistory map[string][]float64
	profits        map[string]float64
	coins          []string
	chartColors    []cell.Color
	mu             sync.RWMutex
}

func newArbitrageDashboard(coins []string) *ArbitrageDashboard {
	return &ArbitrageDashboard{
		coins: coins,
		chartColors: []cell.Color{
			cell.ColorGreen,
			cell.ColorBlue,
			cell.ColorCyan,
			cell.ColorMagenta,
			cell.ColorYellow,
		},
		coinWidgets:    make(map[string]*text.Text),
		updateChan:     make(chan CoinData, 100),
		closeChan:      make(chan struct{}),
		spreadsHistory: make(map[string][]float64),
		profits:        make(map[string]float64),
	}
}

func (ad *ArbitrageDashboard) initWidgets() error {
	// Initialize coin widgets
	for _, coin := range ad.coins {
		widget, err := text.New(text.RollContent(), text.WrapAtWords())
		if err != nil {
			return fmt.Errorf("failed to create text widget for %s: %v", coin, err)
		}
		ad.coinWidgets[coin] = widget
	}

	// Initialize bar chart
	barChart, err := barchart.New(
		barchart.BarColors(ad.chartColors),
		barchart.ShowValues(),
		barchart.Labels(ad.coins),
	)
	if err != nil {
		return fmt.Errorf("failed to create bar chart: %v", err)
	}
	ad.barChart = barChart

	// Initialize line chart
	lineChart, err := linechart.New(
		linechart.AxesCellOpts(cell.FgColor(cell.ColorWhite)),
	)
	if err != nil {
		return fmt.Errorf("failed to create line chart: %v", err)
	}
	ad.lineChart = lineChart

	return nil
}

func (ad *ArbitrageDashboard) startUpdateListener(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case coinData := <-ad.updateChan:
				ad.processCoinUpdate(coinData)
			}
		}
	}()
}

func (ad *ArbitrageDashboard) processCoinUpdate(coinData CoinData) {
	ad.mu.Lock()
	defer ad.mu.Unlock()

	// Update coin widget
	widget, exists := ad.coinWidgets[coinData.Symbol]
	if exists {
		widget.Reset()
		widget.Write(fmt.Sprintf(`
Buy Exchange: %s
Buy Price:    $%.7f
Sell Exchange: %s
Sell Price:    $%.7f
Profit:        %.7f%%
Spread:        %.7f%%
Time:          %s
`,
			coinData.BuyExchange, coinData.BuyPrice,
			coinData.SellExchange, coinData.SellPrice,
			coinData.Profit, coinData.Spread,
			coinData.Timestamp.Format("15:04:05"),
		))
	}

	// Update profits
	ad.profits[coinData.Symbol] = coinData.Profit

	// Update spread history
	if len(ad.spreadsHistory[coinData.Symbol]) >= maxHistorySize {
		ad.spreadsHistory[coinData.Symbol] = ad.spreadsHistory[coinData.Symbol][1:]
	}
	ad.spreadsHistory[coinData.Symbol] = append(ad.spreadsHistory[coinData.Symbol], coinData.Spread)

	// Update bar chart
	barData := make([]int, len(ad.coins))
	for i, coin := range ad.coins {
		barData[i] = int(math.Abs(ad.profits[coin]) * 20000)
	}
	ad.barChart.Values(barData, 1000)

	// Update line chart
	for _, coin := range ad.coins {
		if coinSpreads, ok := ad.spreadsHistory[coin]; ok && len(coinSpreads) > 0 {
			colorIndex := 0
			for i, c := range ad.coins {
				if c == coin {
					colorIndex = i
					break
				}
			}
			ad.lineChart.Series(
				coin,
				coinSpreads,
				linechart.SeriesCellOpts(cell.FgColor(ad.chartColors[colorIndex])),
			)
		}
	}
}

func createGridLayout(ad *ArbitrageDashboard) ([]container.Option, error) {
	builder := grid.New()

	builder.Add(
		grid.RowHeightPerc(25,
			createCoinWidgetsRow(ad.coinWidgets, ad.coins)...,
		),
		grid.RowHeightPerc(75,
			grid.ColWidthPerc(50,
				grid.Widget(ad.barChart,
					container.Border(linestyle.Light),
					container.BorderTitle(" Arbitrage Profits "),
				),
			),
			grid.ColWidthPerc(50,
				grid.Widget(ad.lineChart,
					container.Border(linestyle.Light),
					container.BorderTitle(" Arbitrage Spread History "),
				),
			),
		),
	)

	return builder.Build()
}

func createCoinWidgetsRow(coinWidgets map[string]*text.Text, coins []string) []grid.Element {
	var elements []grid.Element
	for _, coin := range coins {
		elements = append(elements,
			grid.ColWidthPerc(20,
				grid.Widget(coinWidgets[coin],
					container.Border(linestyle.Light),
					container.BorderTitle(fmt.Sprintf(" %s Arbitrage ", coin)),
				),
			),
		)
	}
	return elements
}

func runArbitrageMonitoring(ctx context.Context, ad *ArbitrageDashboard, coins []string) {
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
						ad.updateChan <- CoinData{
							Symbol:       symbol,
							BuyExchange:  lowestAsk.Exchange,
							BuyPrice:     lowestAsk.Price,
							SellExchange: highestBid.Exchange,
							SellPrice:    highestBid.Price,
							Profit:       profit,
							Spread:       spread,
							Timestamp:    time.Now(),
						}
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

	// Initialize terminal
	t, err := tcell.New(tcell.ColorMode(terminalapi.ColorMode256))
	if err != nil {
		log.Fatalf("Failed to initialize terminal: %v", err)
	}
	defer t.Close()

	// Create Arbitrage Dashboard
	ad := newArbitrageDashboard(coins)

	// Initialize widgets
	if err := ad.initWidgets(); err != nil {
		log.Fatalf("Failed to initialize widgets: %v", err)
	}

	// Start update listener
	ad.startUpdateListener(ctx)

	// Build grid layout
	gridOpts, err := createGridLayout(ad)
	if err != nil {
		log.Fatalf("Failed to build grid layout: %v", err)
	}

	// Create the root container with the grid layout
	c, err := container.New(t, gridOpts...)
	if err != nil {
		log.Fatalf("Failed to create root container: %v", err)
	}

	// Run arbitrage monitoring in parallel with terminal dashboard
	go runArbitrageMonitoring(ctx, ad, coins)

	// Run the terminal dashboard
	if err := termdash.Run(ctx, t, c, termdash.RedrawInterval(redrawInterval)); err != nil {
		log.Fatalf("Failed to run termdash: %v", err)
	}

	// Keep the program running until the context is canceled
	<-ctx.Done()
	log.Println("Program exited gracefully.")
}
