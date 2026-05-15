package service

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
)

// ---------------------------------------------------------------------------
// Decision Tree (CART) — Training + Inference
// ---------------------------------------------------------------------------

// TreeNode represents a node in a decision tree.
type TreeNode struct {
	FeatureIndex int       `json:"f,omitempty"` // split feature index (-1 for leaf)
	Threshold    float64   `json:"t,omitempty"` // split threshold
	Left         *TreeNode `json:"l,omitempty"` // <= threshold
	Right        *TreeNode `json:"r,omitempty"` // > threshold
	LeafValue    string    `json:"v,omitempty"` // class label (for classification leaf)
	LeafProba    float64   `json:"p,omitempty"` // probability of positive class (for RF)
}

// Predict traverses the tree and returns the leaf class label.
func (n *TreeNode) Predict(features []float64) string {
	if n == nil {
		return ""
	}
	if n.Left == nil && n.Right == nil {
		return n.LeafValue
	}
	if n.FeatureIndex < 0 || n.FeatureIndex >= len(features) {
		return n.LeafValue
	}
	if features[n.FeatureIndex] <= n.Threshold {
		if n.Left == nil {
			return n.LeafValue
		}
		return n.Left.Predict(features)
	}
	if n.Right == nil {
		return n.LeafValue
	}
	return n.Right.Predict(features)
}

// PredictProba traverses the tree and returns the positive class probability.
func (n *TreeNode) PredictProba(features []float64) float64 {
	if n == nil {
		return 0.5
	}
	if n.Left == nil && n.Right == nil {
		return n.LeafProba
	}
	if n.FeatureIndex < 0 || n.FeatureIndex >= len(features) {
		return 0.5
	}
	if features[n.FeatureIndex] <= n.Threshold {
		if n.Left == nil {
			return 0.5
		}
		return n.Left.PredictProba(features)
	}
	if n.Right == nil {
		return 0.5
	}
	return n.Right.PredictProba(features)
}

// TrainDecisionTree trains a CART decision tree for classification.
func TrainDecisionTree(X [][]float64, y []string, maxDepth int, rng *rand.Rand) *TreeNode {
	return buildTree(X, y, 0, maxDepth, rng)
}

func buildTree(X [][]float64, y []string, depth, maxDepth int, rng *rand.Rand) *TreeNode {
	if len(X) == 0 || len(y) == 0 {
		return &TreeNode{
			FeatureIndex: -1,
			LeafValue:    majorityClass(y),
			LeafProba:    positiveRatio(y),
		}
	}
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
	Trees       []*TreeNode `json:"trees"`
	NEstimators int         `json:"n_estimators"`
}

// TrainRandomForest trains a random forest classifier.
func TrainRandomForest(X [][]float64, y []string, nEstimators, maxDepth int, rng *rand.Rand) *RandomForest {
	if len(X) == 0 || len(y) == 0 || nEstimators <= 0 {
		return &RandomForest{}
	}
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
	if rf == nil || len(rf.Trees) == 0 {
		return 0.5
	}
	sum := 0.0
	count := 0
	for _, tree := range rf.Trees {
		if tree == nil {
			continue
		}
		sum += tree.PredictProba(features)
		count++
	}
	if count == 0 {
		return 0.5
	}
	return sum / float64(count)
}

// Predict returns the majority class prediction.
func (rf *RandomForest) Predict(features []float64) string {
	if rf == nil || len(rf.Trees) == 0 {
		return "0"
	}
	votes := make(map[string]int)
	for _, tree := range rf.Trees {
		if tree == nil {
			continue
		}
		pred := tree.Predict(features)
		if pred == "" {
			continue
		}
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
	TimingTree   *TreeNode     `json:"timing_tree"`
	RFModel      *RandomForest `json:"rf_model"`
	FeatureNames []string      `json:"feature_names"`
	Medians      []float64     `json:"medians"` // for imputation
	Version      string        `json:"version"`
	TrainedAt    string        `json:"trained_at"`
	TimingLOOCV  float64       `json:"timing_loocv,omitempty"` // legacy name; value is LOOCV accuracy
	RFAccuracy   float64       `json:"rf_accuracy,omitempty"`
	RFAUC        float64       `json:"rf_auc,omitempty"` // legacy compatibility for existing model JSON
}

// SaveModelBundle writes the model bundle to a JSON file.
func SaveModelBundle(bundle *PretouchModelBundle, path string) error {
	if err := validatePretouchModelBundle(bundle); err != nil {
		return err
	}
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
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
	if bundle.RFAccuracy == 0 && bundle.RFAUC > 0 {
		bundle.RFAccuracy = bundle.RFAUC
	}
	if err := validatePretouchModelBundle(&bundle); err != nil {
		return nil, err
	}
	return &bundle, nil
}

func validatePretouchModelBundle(bundle *PretouchModelBundle) error {
	if bundle == nil {
		return fmt.Errorf("pretouch model bundle is nil")
	}
	if bundle.TimingTree == nil {
		return fmt.Errorf("pretouch model missing timing_tree")
	}
	if bundle.RFModel == nil {
		return fmt.Errorf("pretouch model missing rf_model")
	}
	if len(bundle.FeatureNames) == 0 {
		return fmt.Errorf("pretouch model missing feature_names")
	}
	if len(bundle.Medians) != len(bundle.FeatureNames) {
		return fmt.Errorf("pretouch model medians length %d does not match feature_names length %d", len(bundle.Medians), len(bundle.FeatureNames))
	}
	if maxIndex := maxTreeFeatureIndex(bundle.TimingTree); maxIndex >= len(bundle.FeatureNames) {
		return fmt.Errorf("pretouch timing_tree feature index %d exceeds feature_names length %d", maxIndex, len(bundle.FeatureNames))
	}
	for i, tree := range bundle.RFModel.Trees {
		if tree == nil {
			continue
		}
		if maxIndex := maxTreeFeatureIndex(tree); maxIndex >= len(bundle.FeatureNames) {
			return fmt.Errorf("pretouch rf_model tree %d feature index %d exceeds feature_names length %d", i, maxIndex, len(bundle.FeatureNames))
		}
	}
	return nil
}

func maxTreeFeatureIndex(node *TreeNode) int {
	if node == nil {
		return -1
	}
	maxIndex := -1
	if node.Left != nil || node.Right != nil {
		maxIndex = node.FeatureIndex
	}
	if left := maxTreeFeatureIndex(node.Left); left > maxIndex {
		maxIndex = left
	}
	if right := maxTreeFeatureIndex(node.Right); right > maxIndex {
		maxIndex = right
	}
	return maxIndex
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
