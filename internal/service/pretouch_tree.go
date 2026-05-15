package service

import (
	"encoding/json"
	"math"
	"math/rand"
	"os"
	"sort"
)

// ---------------------------------------------------------------------------
// Decision Tree (CART) — Training + Inference
// ---------------------------------------------------------------------------

// TreeNode represents a node in a decision tree.
type TreeNode struct {
	FeatureIndex int      `json:"f,omitempty"`  // split feature index (-1 for leaf)
	Threshold    float64  `json:"t,omitempty"`  // split threshold
	Left         *TreeNode `json:"l,omitempty"` // <= threshold
	Right        *TreeNode `json:"r,omitempty"` // > threshold
	LeafValue    string   `json:"v,omitempty"`  // class label (for classification leaf)
	LeafProba    float64  `json:"p,omitempty"`  // probability of positive class (for RF)
}

// Predict traverses the tree and returns the leaf class label.
func (n *TreeNode) Predict(features []float64) string {
	if n.Left == nil && n.Right == nil {
		return n.LeafValue
	}
	if features[n.FeatureIndex] <= n.Threshold {
		return n.Left.Predict(features)
	}
	return n.Right.Predict(features)
}

// PredictProba traverses the tree and returns the positive class probability.
func (n *TreeNode) PredictProba(features []float64) float64 {
	if n.Left == nil && n.Right == nil {
		return n.LeafProba
	}
	if features[n.FeatureIndex] <= n.Threshold {
		return n.Left.PredictProba(features)
	}
	return n.Right.PredictProba(features)
}

// TrainDecisionTree trains a CART decision tree for classification.
func TrainDecisionTree(X [][]float64, y []string, maxDepth int, rng *rand.Rand) *TreeNode {
	return buildTree(X, y, 0, maxDepth, rng)
}

func buildTree(X [][]float64, y []string, depth, maxDepth int, rng *rand.Rand) *TreeNode {
	// Check stopping conditions
	if depth >= maxDepth || len(y) <= 1 || allSame(y) {
		return &TreeNode{
			FeatureIndex: -1,
			LeafValue:    majorityClass(y),
			LeafProba:    positiveRatio(y),
		}
	}

	// Find best split
	bestFeature, bestThreshold, bestGain := -1, 0.0, 0.0
	nFeatures := len(X[0])

	for f := 0; f < nFeatures; f++ {
		thresholds := uniqueThresholds(X, f)
		for _, t := range thresholds {
			gain := giniGain(X, y, f, t)
			if gain > bestGain {
				bestGain = gain
				bestFeature = f
				bestThreshold = t
			}
		}
	}

	if bestFeature == -1 || bestGain <= 0 {
		return &TreeNode{
			FeatureIndex: -1,
			LeafValue:    majorityClass(y),
			LeafProba:    positiveRatio(y),
		}
	}

	// Split data
	leftX, leftY, rightX, rightY := splitData(X, y, bestFeature, bestThreshold)

	return &TreeNode{
		FeatureIndex: bestFeature,
		Threshold:    bestThreshold,
		Left:         buildTree(leftX, leftY, depth+1, maxDepth, rng),
		Right:        buildTree(rightX, rightY, depth+1, maxDepth, rng),
	}
}

// ---------------------------------------------------------------------------
// Random Forest — Training + Inference
// ---------------------------------------------------------------------------

// RandomForest is an ensemble of decision trees.
type RandomForest struct {
	Trees      []*TreeNode `json:"trees"`
	NEstimators int        `json:"n_estimators"`
}

// TrainRandomForest trains a random forest classifier.
func TrainRandomForest(X [][]float64, y []string, nEstimators, maxDepth int, rng *rand.Rand) *RandomForest {
	trees := make([]*TreeNode, nEstimators)
	n := len(X)

	for i := 0; i < nEstimators; i++ {
		// Bootstrap sample
		indices := make([]int, n)
		for j := range indices {
			indices[j] = rng.Intn(n)
		}

		bootX := make([][]float64, n)
		bootY := make([]string, n)
		for j, idx := range indices {
			bootX[j] = X[idx]
			bootY[j] = y[idx]
		}

		trees[i] = TrainDecisionTree(bootX, bootY, maxDepth, rng)
	}

	return &RandomForest{Trees: trees, NEstimators: nEstimators}
}

// PredictProba returns the probability of the positive class ("1").
func (rf *RandomForest) PredictProba(features []float64) float64 {
	if len(rf.Trees) == 0 {
		return 0.5
	}
	sum := 0.0
	for _, tree := range rf.Trees {
		sum += tree.PredictProba(features)
	}
	return sum / float64(len(rf.Trees))
}

// Predict returns the majority class prediction.
func (rf *RandomForest) Predict(features []float64) string {
	if len(rf.Trees) == 0 {
		return "0"
	}
	votes := make(map[string]int)
	for _, tree := range rf.Trees {
		pred := tree.Predict(features)
		votes[pred]++
	}
	best := ""
	bestCount := 0
	for cls, count := range votes {
		if count > bestCount {
			best = cls
			bestCount = count
		}
	}
	return best
}

// ---------------------------------------------------------------------------
// Model Persistence (JSON)
// ---------------------------------------------------------------------------

// PretouchModelBundle holds both timing classifier and RF model.
type PretouchModelBundle struct {
	TimingTree    *TreeNode     `json:"timing_tree"`
	RFModel       *RandomForest `json:"rf_model"`
	FeatureNames  []string      `json:"feature_names"`
	Medians       []float64     `json:"medians"` // for imputation
	Version       string        `json:"version"`
	TrainedAt     string        `json:"trained_at"`
	TimingLOOCV   float64       `json:"timing_loocv"`
	RFAUC         float64       `json:"rf_auc"`
}

// SaveModelBundle writes the model bundle to a JSON file.
func SaveModelBundle(bundle *PretouchModelBundle, path string) error {
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadModelBundle reads a model bundle from a JSON file.
func LoadModelBundle(path string) (*PretouchModelBundle, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var bundle PretouchModelBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return nil, err
	}
	return &bundle, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func allSame(y []string) bool {
	if len(y) == 0 {
		return true
	}
	first := y[0]
	for _, v := range y[1:] {
		if v != first {
			return false
		}
	}
	return true
}

func majorityClass(y []string) string {
	counts := make(map[string]int)
	for _, v := range y {
		counts[v]++
	}
	best := ""
	bestCount := 0
	for cls, count := range counts {
		if count > bestCount || (count == bestCount && cls < best) {
			best = cls
			bestCount = count
		}
	}
	return best
}

func positiveRatio(y []string) float64 {
	if len(y) == 0 {
		return 0.5
	}
	pos := 0
	for _, v := range y {
		if v == "1" || v == "fast" || v == "slow" {
			pos++
		}
	}
	return float64(pos) / float64(len(y))
}

func giniImpurity(y []string) float64 {
	if len(y) == 0 {
		return 0
	}
	counts := make(map[string]int)
	for _, v := range y {
		counts[v]++
	}
	n := float64(len(y))
	impurity := 1.0
	for _, count := range counts {
		p := float64(count) / n
		impurity -= p * p
	}
	return impurity
}

func giniGain(X [][]float64, y []string, featureIdx int, threshold float64) float64 {
	parentGini := giniImpurity(y)

	var leftY, rightY []string
	for i, row := range X {
		if row[featureIdx] <= threshold {
			leftY = append(leftY, y[i])
		} else {
			rightY = append(rightY, y[i])
		}
	}

	if len(leftY) == 0 || len(rightY) == 0 {
		return 0
	}

	n := float64(len(y))
	leftWeight := float64(len(leftY)) / n
	rightWeight := float64(len(rightY)) / n

	return parentGini - leftWeight*giniImpurity(leftY) - rightWeight*giniImpurity(rightY)
}

func uniqueThresholds(X [][]float64, featureIdx int) []float64 {
	vals := make([]float64, len(X))
	for i, row := range X {
		vals[i] = row[featureIdx]
	}
	sort.Float64s(vals)

	// Midpoints between unique sorted values
	thresholds := make([]float64, 0, len(vals))
	for i := 1; i < len(vals); i++ {
		if vals[i] != vals[i-1] {
			thresholds = append(thresholds, (vals[i]+vals[i-1])/2)
		}
	}

	// Limit to at most 50 thresholds for performance
	if len(thresholds) > 50 {
		step := len(thresholds) / 50
		sampled := make([]float64, 0, 50)
		for i := 0; i < len(thresholds); i += step {
			sampled = append(sampled, thresholds[i])
		}
		return sampled
	}
	return thresholds
}

func splitData(X [][]float64, y []string, featureIdx int, threshold float64) ([][]float64, []string, [][]float64, []string) {
	var leftX, rightX [][]float64
	var leftY, rightY []string
	for i, row := range X {
		if row[featureIdx] <= threshold {
			leftX = append(leftX, row)
			leftY = append(leftY, y[i])
		} else {
			rightX = append(rightX, row)
			rightY = append(rightY, y[i])
		}
	}
	return leftX, leftY, rightX, rightY
}

// Suppress unused import warning for math
var _ = math.Abs
