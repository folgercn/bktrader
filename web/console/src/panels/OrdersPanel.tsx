import { formatTime, formatNumber, formatMoney, formatMaybeNumber } from '../utils/format';
import { runtimeReadinessTone, summarizeOrderPreflight, buildSignalActionNotes, buildSignalBarStateNotes, buildRuntimeEventNotes } from '../utils/derivation';
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
          <p className="panel-kicker">Orders</p>
          <h3>最新订单</h3>
        </div>
      </div>
      <div className="grid grid-cols-2 gap-8 items-start">
        <div className="backtest-form session-form">
          <h4>Create Live Order</h4>
          <div className="form-grid">
            <label className="form-field">
              <span>Live Account</span>
              <select
                value={liveOrderForm.accountId}
                onChange={(event) => setLiveOrderForm((current) => ({ ...current, accountId: event.target.value }))}
              >
                {liveAccounts.map((account) => (
                  <option key={account.id} value={account.id}>
                    {account.name} ({account.status})
                  </option>
                ))}
              </select>
            </label>
            <label className="form-field">
              <span>Strategy Version</span>
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
              <span>Symbol</span>
              <input
                value={liveOrderForm.symbol}
                onChange={(event) => setLiveOrderForm((current) => ({ ...current, symbol: event.target.value.toUpperCase() }))}
              />
            </label>
            <label className="form-field">
              <span>Side</span>
              <select
                value={liveOrderForm.side}
                onChange={(event) => setLiveOrderForm((current) => ({ ...current, side: event.target.value }))}
              >
                <option value="BUY">BUY</option>
                <option value="SELL">SELL</option>
              </select>
            </label>
            <label className="form-field">
              <span>Type</span>
              <select
                value={liveOrderForm.type}
                onChange={(event) => setLiveOrderForm((current) => ({ ...current, type: event.target.value }))}
              >
                <option value="LIMIT">LIMIT</option>
                <option value="MARKET">MARKET</option>
              </select>
            </label>
            <label className="form-field">
              <span>Quantity</span>
              <input
                value={liveOrderForm.quantity}
                onChange={(event) => setLiveOrderForm((current) => ({ ...current, quantity: event.target.value }))}
              />
            </label>
            <label className="form-field">
              <span>Price</span>
              <input
                value={liveOrderForm.price}
                onChange={(event) => setLiveOrderForm((current) => ({ ...current, price: event.target.value }))}
                placeholder={liveOrderForm.type === "MARKET" ? "optional" : "required for limit"}
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
              label={liveOrderAction ? "Submitting..." : "Submit Live Order"}
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
          <h4>Live Execution Context</h4>
          <div className="detail-grid">
            <div className="detail-item">
              <span>Runtime</span>
              <strong>{selectedLiveOrderActiveRuntime ? `${selectedLiveOrderActiveRuntime.status} · ${selectedLiveOrderActiveRuntime.runtimeAdapter}` : "--"}</strong>
            </div>
            <div className="detail-item">
              <span>Health</span>
              <strong>{String(selectedLiveOrderRuntimeState.health ?? "--")}</strong>
            </div>
            <div className="detail-item">
              <span>Signal Bias</span>
              <strong>{selectedLiveOrderSignalAction.bias}</strong>
            </div>
            <div className="detail-item">
              <span>Signal State</span>
              <strong>{selectedLiveOrderSignalAction.state}</strong>
            </div>
            <div className="detail-item">
              <span>Trade</span>
              <strong>{formatMaybeNumber(selectedLiveOrderMarket.tradePrice)}</strong>
            </div>
            <div className="detail-item">
              <span>Bid / Ask</span>
              <strong>{formatMaybeNumber(selectedLiveOrderMarket.bestBid)} / {formatMaybeNumber(selectedLiveOrderMarket.bestAsk)}</strong>
            </div>
            <div className="detail-item">
              <span>Spread</span>
              <strong>{formatMaybeNumber(selectedLiveOrderMarket.spreadBps)} bps</strong>
            </div>
            <div className="detail-item">
              <span>Signal TF</span>
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
        columns={["Time", "Symbol", "Side", "Qty", "Price", "Status", "Mode", "Runtime", "Preflight"]}
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
            order.status,
            String(order.metadata?.executionMode ?? "--"),
            String(order.metadata?.runtimeSessionId ?? "--"),
            summarizeOrderPreflight(order.metadata?.runtimePreflight),
          ])}
        emptyMessage="No orders"
      />
    </article>
  );
}
