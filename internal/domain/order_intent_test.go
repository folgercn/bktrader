package domain

import "testing"

// Golden Case 回归测试 — 基于 2026-04-29 真实订单流排查建立。
//
// lastVerifiedAt: 2026-04-30
// source: Issue #344 — 工程化梳理交易模块
//
// 新增 signalKind 或修改订单方向判断逻辑时，必须同步更新此测试。
// CI 中 go test ./internal/domain/... -run TestClassifyOrderIntent 为 Required Check。

func TestClassifyOrderIntent_GoldenCases(t *testing.T) {
	cases := []struct {
		name     string
		order    Order
		expected OrderIntent
		display  string // 期望的中文标签
	}{
		// === 组 A：开多 → 平多 ===
		{
			name:     "order-1777471086379688754: BUY zero-initial-reentry → OPEN_LONG",
			order:    Order{Side: "BUY", ReduceOnly: false},
			expected: OrderIntentOpenLong,
			display:  "开多",
		},
		{
			name:     "order-1777471148180758130: SELL risk-exit reduceOnly → CLOSE_LONG",
			order:    Order{Side: "SELL", ReduceOnly: true},
			expected: OrderIntentCloseLong,
			display:  "平多",
		},
		{
			name:     "order-1777471215130841633: BUY zero-initial-reentry → OPEN_LONG",
			order:    Order{Side: "BUY", ReduceOnly: false},
			expected: OrderIntentOpenLong,
			display:  "开多",
		},
		{
			name:     "order-1777471569961949714: SELL risk-exit reduceOnly → CLOSE_LONG",
			order:    Order{Side: "SELL", ReduceOnly: true},
			expected: OrderIntentCloseLong,
			display:  "平多",
		},

		// === 组 B：开空 → 平空 ===
		{
			name:     "order-1777486182526885338: SELL entry → OPEN_SHORT",
			order:    Order{Side: "SELL", ReduceOnly: false},
			expected: OrderIntentOpenShort,
			display:  "开空",
		},
		{
			name:     "order-1777486257764774887: BUY reduceOnly → CLOSE_SHORT",
			order:    Order{Side: "BUY", ReduceOnly: true},
			expected: OrderIntentCloseShort,
			display:  "平空",
		},

		// === 组 C：取消订单（intent 仍然可分类）===
		{
			name:     "order-1777475288746567463: BUY CANCELLED but intent = OPEN_LONG",
			order:    Order{Side: "BUY", ReduceOnly: false, Status: "CANCELLED"},
			expected: OrderIntentOpenLong,
			display:  "开多",
		},

		// === 组 D：closePosition 变体 ===
		{
			name:     "SELL closePosition=true → CLOSE_LONG",
			order:    Order{Side: "SELL", ClosePosition: true},
			expected: OrderIntentCloseLong,
			display:  "平多",
		},
		{
			name:     "BUY closePosition=true → CLOSE_SHORT",
			order:    Order{Side: "BUY", ClosePosition: true},
			expected: OrderIntentCloseShort,
			display:  "平空",
		},

		// === 组 E：Metadata 中的 reduceOnly ===
		{
			name: "SELL with metadata reduceOnly=true → CLOSE_LONG",
			order: Order{
				Side:     "SELL",
				Metadata: map[string]any{"reduceOnly": true},
			},
			expected: OrderIntentCloseLong,
			display:  "平多",
		},
		{
			name: "BUY with metadata closePosition=true → CLOSE_SHORT",
			order: Order{
				Side:     "BUY",
				Metadata: map[string]any{"closePosition": true},
			},
			expected: OrderIntentCloseShort,
			display:  "平空",
		},

		// === 组 F：Metadata 中 string 类型的标志位（覆盖 orderFlagValue 的 string 分支）===
		{
			name: "SELL with metadata reduceOnly=\"true\" (string) → CLOSE_LONG",
			order: Order{
				Side:     "SELL",
				Metadata: map[string]any{"reduceOnly": "true"},
			},
			expected: OrderIntentCloseLong,
			display:  "平多",
		},
		{
			name: "BUY with metadata closePosition=\" true \" (string with spaces) → CLOSE_SHORT",
			order: Order{
				Side:     "BUY",
				Metadata: map[string]any{"closePosition": " true "},
			},
			expected: OrderIntentCloseShort,
			display:  "平空",
		},

		// === 边界 case ===
		{
			name:     "空 side → UNKNOWN",
			order:    Order{Side: ""},
			expected: OrderIntentUnknown,
			display:  "未知",
		},
		{
			name:     "side 含空格 → 仍可分类",
			order:    Order{Side: "  buy  ", ReduceOnly: false},
			expected: OrderIntentOpenLong,
			display:  "开多",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyOrderIntent(tc.order)
			if got != tc.expected {
				t.Fatalf("ClassifyOrderIntent() = %s, want %s", got, tc.expected)
			}
			if got.IntentLabel() != tc.display {
				t.Fatalf("IntentLabel() = %s, want %s", got.IntentLabel(), tc.display)
			}
		})
	}
}

func TestOrderIntent_IsEntry(t *testing.T) {
	if !OrderIntentOpenLong.IsEntry() {
		t.Fatal("OPEN_LONG should be entry")
	}
	if !OrderIntentOpenShort.IsEntry() {
		t.Fatal("OPEN_SHORT should be entry")
	}
	if OrderIntentCloseLong.IsEntry() {
		t.Fatal("CLOSE_LONG should not be entry")
	}
	if OrderIntentCloseShort.IsEntry() {
		t.Fatal("CLOSE_SHORT should not be entry")
	}
	if OrderIntentUnknown.IsEntry() {
		t.Fatal("UNKNOWN should not be entry")
	}
}

func TestOrderIntent_IsExit(t *testing.T) {
	if OrderIntentOpenLong.IsExit() {
		t.Fatal("OPEN_LONG should not be exit")
	}
	if OrderIntentOpenShort.IsExit() {
		t.Fatal("OPEN_SHORT should not be exit")
	}
	if !OrderIntentCloseLong.IsExit() {
		t.Fatal("CLOSE_LONG should be exit")
	}
	if !OrderIntentCloseShort.IsExit() {
		t.Fatal("CLOSE_SHORT should be exit")
	}
	if OrderIntentUnknown.IsExit() {
		t.Fatal("UNKNOWN should not be exit")
	}
}
