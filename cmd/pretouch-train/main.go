// Command pretouch-train trains the ETH pretouch timing model (DT3 + RF)
// from the canonical events CSV and outputs data/pretouch_model.json.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/wuyaocheng/bktrader/internal/service"
)

func main() {
	config := service.DefaultPretouchTrainerConfig()
	flag.StringVar(&config.EventsCSVPath, "events-csv", config.EventsCSVPath, "path to pretouch event CSV")
	flag.StringVar(&config.ModelOutPath, "out", config.ModelOutPath, "path to write model JSON")
	flag.StringVar(&config.ForwardStart, "forward-start", config.ForwardStart, "forward validation start date, YYYY-MM-DD")
	flag.Float64Var(&config.TrainRatio, "train-ratio", config.TrainRatio, "chronological train split ratio")
	flag.IntVar(&config.MaxDepthDT, "dt-depth", config.MaxDepthDT, "timing decision tree max depth")
	flag.IntVar(&config.NEstimatorsRF, "rf-estimators", config.NEstimatorsRF, "random forest estimator count")
	flag.Int64Var(&config.RandomSeed, "seed", config.RandomSeed, "random seed")
	flag.Parse()

	fmt.Printf("=== Pretouch Model Training ===\n")
	fmt.Printf("Events CSV: %s\n", config.EventsCSVPath)
	fmt.Printf("Model output: %s\n", config.ModelOutPath)
	fmt.Printf("Forward start: %s\n", config.ForwardStart)
	fmt.Printf("Train ratio: %.2f\n", config.TrainRatio)
	fmt.Printf("DT max depth: %d\n", config.MaxDepthDT)
	fmt.Printf("RF estimators: %d\n", config.NEstimatorsRF)
	fmt.Printf("Random seed: %d\n", config.RandomSeed)
	fmt.Println()

	start := time.Now()
	if err := service.TrainPretouchModel(config); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	elapsed := time.Since(start)
	fmt.Printf("\nModel trained successfully in %s\n", elapsed)
	fmt.Printf("Output: %s\n", config.ModelOutPath)
}
