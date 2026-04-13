import { formatTime, formatNumber, formatMoney, formatMaybeNumber } from '../utils/format';
import { runtimeReadinessTone, summarizeOrderPreflight, buildSignalActionNotes, buildSignalBarStateNotes, buildRuntimeEventNotes, technicalStatusLabel } from '../utils/derivation';
import { AccountRecord, StrategyRecord, Order, LiveOrderForm, LivePreflightSummary, SignalRuntimeSession, RuntimeMarketSnapshot } from '../types/domain';
import { StatusPill } from '../components/ui/StatusPill';
import { ActionButton } from '../components/ui/ActionButton';
import { SimpleTable } from '../components/ui/SimpleTable';

interface OrdersPanelProps {
  liveOrderForm: LiveOrderForm;
  setLiveOrderForm: (valOrUpdater: LiveOrderForm | ((prev: LiveOrderForm) => LiveOrderForm)) => void;
  liveAccounts: AccountRecord[];
  strategies: StrategyRecord[];
  selectedLiveOrderPreflight: any;
  liveOrderAction: boolean;
  createLiveOrder: () => void;
  selectedLiveOrderActiveRuntime: SignalRuntimeSession | null;
  selectedLiveOrderRuntimeState: Record<string, any>; // Derivation result, complex object
  selectedLiveOrderSignalAction: Record<string, any>; // Derivation result, complex object
  selectedLiveOrderMarket: RuntimeMarketSnapshot;
  selectedLiveOrderSignalBarState: Record<string, any>; // Derivation result, complex object
  selectedLiveOrderRuntimeSummary: Record<string, any>; // Derivation result, complex object
  orders: Order[];
}

export function OrdersPanel({
  liveOrderForm,
  setLiveOrderForm,
  liveAccounts,
  strategies,
  selectedLiveOrderPreflight,
  liveOrderAction,
  createLiveOrder,
  selectedLiveOrderActiveRuntime,
  selectedLiveOrderRuntimeState,
  selectedLiveOrderSignalAction,
  selectedLiveOrderMarket,
  selectedLiveOrderSignalBarState,
  selectedLiveOrderRuntimeSummary,
  orders
}: OrdersPanelProps) {
  return (
    <article id="orders" className="panel">
      <div className="panel-header">
        <div>
          <p className="panel-kicker">订单</p>
          <h3>最新订单</h3>
        </div>
      </div>
      <div className="grid grid-cols-2 gap-8 items-start">
        <div className="backtest-form session-form">
          <h4>创建实盘订单</h4>
          <div className="form-grid">
            <label className="form-field">
              <span>实盘账户</span>
              <select
                value={liveOrderForm.accountId}
                onChange={(event) => setLiveOrderForm((current) => ({ ...current, accountId: event.target.value }))}
              >
                {liveAccounts.map((account) => (
                  <option key={account.id} value={account.id}>
                    {account.name} ({technicalStatusLabel(account.status)})
                  </option>
                ))}
              </select>
            </label>
            <label className="form-field">
              <span>策略版本</span>
              <select
                value={liveOrderForm.strategyVersionId}
                onChange={(event) => setLiveOrderForm((current) => ({ ...current, strategyVersionId: event.target.value }))}
              >
                <option value="">Auto</option>
                {strategies.map((strategy) => (
                  <option key={strategy.id} value={strategy.currentVersion?.id ?? ""}>
                    {strategy.name} · {strategy.currentVersion?.version ?? "no-version"}
                  </option>
                ))}
              </select>
            </label>
            <label className="form-field">
              <span>代码</span>
              <input
                value={liveOrderForm.symbol}
                onChange={(event) => setLiveOrderForm((current) => ({ ...current, symbol: event.target.value.toUpperCase() }))}
              />
            </label>
            <label className="form-field">
              <span>方向</span>
              <select
                value={liveOrderForm.side}
                onChange={(event) => setLiveOrderForm((current) => ({ ...current, side: event.target.value }))}
              >
                <option value="BUY">BUY</option>
                <option value="SELL">SELL</option>
              </select>
            </label>
            <label className="form-field">
              <span>类型</span>
              <select
                value={liveOrderForm.type}
                onChange={(event) => setLiveOrderForm((current) => ({ ...current, type: event.target.value }))}
              >
                <option value="LIMIT">LIMIT</option>
                <option value="MARKET">MARKET</option>
              </select>
            </label>
            <label className="form-field">
              <span>数量</span>
              <input
                value={liveOrderForm.quantity}
                onChange={(event) => setLiveOrderForm((current) => ({ ...current, quantity: event.target.value }))}
              />
            </label>
            <label className="form-field">
              <span>价格</span>
              <input
                value={liveOrderForm.price}
                onChange={(event) => setLiveOrderForm((current) => ({ ...current, price: event.target.value }))}
                placeholder={liveOrderForm.type === "MARKET" ? "选填" : "限价单必填"}
              />
            </label>
          </div>
          <div className="live-account-meta">
            <span>
              <StatusPill tone={runtimeReadinessTone(selectedLiveOrderPreflight.status)}>
                {selectedLiveOrderPreflight.status}
              </StatusPill>
            </span>
            <span>{selectedLiveOrderPreflight.reason}</span>
            <span>{selectedLiveOrderPreflight.detail}</span>
          </div>
          <div className="backtest-actions">
            <ActionButton
              label={liveOrderAction ? "提交中..." : "提交实盘订单"}
              disabled={
                liveOrderAction ||
                selectedLiveOrderPreflight.status === "blocked" ||
                !liveOrderForm.accountId ||
                !liveOrderForm.symbol.trim() ||
                !(Number(liveOrderForm.quantity) > 0) ||
                (liveOrderForm.type === "LIMIT" && !(Number(liveOrderForm.price) > 0))
              }
              onClick={createLiveOrder}
            />
          </div>
        </div>

        <div className="backtest-list session-form">
          <h4>实盘执行上下文</h4>
          <div className="detail-grid">
            <div className="detail-item">
              <span>运行时</span>
              <strong>{selectedLiveOrderActiveRuntime ? `${technicalStatusLabel(selectedLiveOrderActiveRuntime.status)} · ${selectedLiveOrderActiveRuntime.runtimeAdapter}` : "--"}</strong>
            </div>
            <div className="detail-item">
              <span>健康度</span>
              <strong>{technicalStatusLabel(selectedLiveOrderRuntimeState.health)}</strong>
            </div>
            <div className="detail-item">
              <span>信号偏向</span>
              <strong>{technicalStatusLabel(selectedLiveOrderSignalAction.bias)}</strong>
            </div>
            <div className="detail-item">
              <span>信号状态</span>
              <strong>{technicalStatusLabel(selectedLiveOrderSignalAction.state)}</strong>
            </div>
            <div className="detail-item">
              <span>成交价</span>
              <strong>{formatMaybeNumber(selectedLiveOrderMarket.tradePrice)}</strong>
            </div>
            <div className="detail-item">
              <span>买 / 卖</span>
              <strong>{formatMaybeNumber(selectedLiveOrderMarket.bestBid)} / {formatMaybeNumber(selectedLiveOrderMarket.bestAsk)}</strong>
            </div>
            <div className="detail-item">
              <span>价差</span>
              <strong>{formatMaybeNumber(selectedLiveOrderMarket.spreadBps)} bps</strong>
            </div>
            <div className="detail-item">
              <span>信号周期</span>
              <strong>{String(selectedLiveOrderSignalBarState.timeframe ?? "--")}</strong>
            </div>
            <div className="detail-item">
              <span>MA20 / ATR14</span>
              <strong>{formatMaybeNumber(selectedLiveOrderSignalBarState.ma20)} / {formatMaybeNumber(selectedLiveOrderSignalBarState.atr14)}</strong>
            </div>
          </div>
          <div className="backtest-notes">
            {buildSignalActionNotes(selectedLiveOrderSignalAction as any).map((line: string) => (
              <div key={line} className="note-item">
                {line}
              </div>
            ))}
            {buildSignalBarStateNotes(selectedLiveOrderSignalBarState).slice(0, 2).map((line: string) => (
              <div key={line} className="note-item">
                {line}
              </div>
            ))}
            {buildRuntimeEventNotes(selectedLiveOrderRuntimeSummary).map((line: string) => (
              <div key={line} className="note-item">
                {line}
              </div>
            ))}
          </div>
        </div>
      </div>
      <SimpleTable
        columns={["时间", "代码", "方向", "数量", "价格", "状态", "模式", "运行时", "预检"]}
        rows={orders
          .slice()
          .reverse()
          .slice(0, 8)
          .map((order) => [
            formatTime(String(order.metadata?.eventTime ?? order.createdAt)),
            order.symbol,
            order.side,
            formatNumber(order.quantity, 4),
            formatMoney(order.price),
            technicalStatusLabel(order.status),
            technicalStatusLabel(order.metadata?.executionMode),
            String(order.metadata?.runtimeSessionId ?? "--"),
            summarizeOrderPreflight(order.metadata?.runtimePreflight),
          ])}
        emptyMessage="暂无订单"
      />
    </article>
  );
}
