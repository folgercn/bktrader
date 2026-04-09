import React, { useMemo } from 'react';
import { useUIStore } from '../store/useUIStore';
import { useTradingStore } from '../store/useTradingStore';
import { ActionButton } from '../components/ui/ActionButton';
import { SimpleTable } from '../components/ui/SimpleTable';
import { formatTime } from '../utils/format';
import { strategyLabel, getRecord } from '../utils/derivation';

interface StrategyStageProps {
  createStrategy: () => void;
  saveStrategyParameters: () => void;
}

export function StrategyStage({ createStrategy, saveStrategyParameters }: StrategyStageProps) {
  const strategies = useTradingStore(s => s.strategies);
  const signalRuntimeAdapters = useTradingStore(s => s.signalRuntimeAdapters);
  const strategyCreateForm = useUIStore(s => s.strategyCreateForm);
  const setStrategyCreateForm = useUIStore(s => s.setStrategyCreateForm);
  const strategyCreateAction = useUIStore(s => s.strategyCreateAction);
  const strategyEditorForm = useUIStore(s => s.strategyEditorForm);
  const setStrategyEditorForm = useUIStore(s => s.setStrategyEditorForm);
  const strategySaveAction = useUIStore(s => s.strategySaveAction);
  const selectedStrategyId = useTradingStore(s => s.selectedStrategyId);
  const setSelectedStrategyId = useTradingStore(s => s.setSelectedStrategyId);

  // Derived states
  const strategyOptions = useMemo(
    () =>
      strategies.map((strategy) => ({
        value: strategy.id,
        label: strategyLabel(strategy),
      })),
    [strategies]
  );

  const selectedStrategy =
    strategies.find((item) => item.id === (selectedStrategyId || strategyEditorForm.strategyId)) ?? strategies[0] ?? null;
  const selectedStrategyVersion = selectedStrategy?.currentVersion ?? null;
  const selectedStrategyParameters = getRecord(selectedStrategyVersion?.parameters);

  return (
    <div className="absolute inset-0 overflow-y-auto p-6 space-y-6 bg-zinc-950/50">
      <section id="strategies" className="panel panel-backtests">
        <div className="panel-header">
          <div>
            <p className="panel-kicker">Strategies</p>
            <h3>策略管理</h3>
          </div>
          <div className="range-box">
            <span>{strategies.length} strategies</span>
            <span>{signalRuntimeAdapters.length} engines</span>
          </div>
        </div>
        <div className="live-grid">
          <div className="backtest-form session-form">
            <h4>创建策略</h4>
            <div className="form-grid">
              <label className="form-field">
                <span>策略名称</span>
                <input
                  value={strategyCreateForm.name}
                  onChange={(event) => setStrategyCreateForm((current: any) => ({ ...current, name: event.target.value }))}
                  placeholder="例如：BK 4H Runner"
                />
              </label>
              <label className="form-field form-field-wide">
                <span>策略说明</span>
                <input
                  value={strategyCreateForm.description}
                  onChange={(event) =>
                    setStrategyCreateForm((current: any) => ({ ...current, description: event.target.value }))
                  }
                  placeholder="记录这条策略的用途、市场和执行方式"
                />
              </label>
            </div>
            <div className="backtest-actions">
              <ActionButton
                label={strategyCreateAction ? "创建中..." : "创建策略"}
                disabled={strategyCreateAction || !strategyCreateForm.name.trim()}
                onClick={createStrategy}
              />
            </div>
            <div className="backtest-notes">
              <div className="note-item">第一版先直接创建当前版本，默认引擎是 bk-default。</div>
              <div className="note-item">版本历史和回滚下一步再补，这一版先保证你能直接改参数。</div>
            </div>
          </div>

          <div className="backtest-form session-form">
            <h4>策略参数编辑</h4>
            <div className="form-grid">
              <label className="form-field">
                <span>选择策略</span>
                <select
                  value={strategyEditorForm.strategyId}
                  onChange={(event) => {
                    setSelectedStrategyId(event.target.value);
                    setStrategyEditorForm((current: any) => ({ ...current, strategyId: event.target.value }));
                  }}
                >
                  {strategyOptions.map((strategy) => (
                    <option key={strategy.value} value={strategy.value}>
                      {strategy.label}
                    </option>
                  ))}
                </select>
              </label>
              <label className="form-field">
                <span>策略引擎</span>
                <select
                  value={strategyEditorForm.strategyEngine}
                  onChange={(event) =>
                    setStrategyEditorForm((current: any) => ({ ...current, strategyEngine: event.target.value }))
                  }
                >
                  {[...new Set(["bk-default", ...strategies.map((item) => String(getRecord(item.currentVersion?.parameters).strategyEngine || "bk-default"))])].map((engineKey) => (
                    <option key={engineKey} value={engineKey}>
                      {engineKey}
                    </option>
                  ))}
                </select>
              </label>
              <label className="form-field">
                <span>信号周期</span>
                <select
                  value={strategyEditorForm.signalTimeframe}
                  onChange={(event) =>
                    setStrategyEditorForm((current: any) => ({ ...current, signalTimeframe: event.target.value }))
                  }
                >
                  <option value="4h">4h</option>
                  <option value="1d">1d</option>
                </select>
              </label>
              <label className="form-field">
                <span>执行数据源</span>
                <select
                  value={strategyEditorForm.executionDataSource}
                  onChange={(event) =>
                    setStrategyEditorForm((current: any) => ({ ...current, executionDataSource: event.target.value }))
                  }
                >
                  <option value="tick">tick</option>
                  <option value="1min">1min</option>
                </select>
              </label>
              <label className="form-field form-field-wide">
                <span>参数 JSON</span>
                <textarea
                  rows={14}
                  value={strategyEditorForm.parametersJson}
                  onChange={(event) =>
                    setStrategyEditorForm((current: any) => ({ ...current, parametersJson: event.target.value }))
                  }
                  placeholder='{"stop_loss_atr":0.05,"profit_protect_atr":1.0}'
                />
              </label>
            </div>
            <div className="backtest-actions inline-actions">
              <ActionButton
                label={strategySaveAction ? "保存中..." : "保存策略参数"}
                disabled={strategySaveAction || !strategyEditorForm.strategyId}
                onClick={saveStrategyParameters}
              />
              <button
                type="button"
                className="filter-chip"
                onClick={() => {
                  if (!selectedStrategy) {
                    return;
                  }
                  const currentParameters = getRecord(selectedStrategy.currentVersion?.parameters);
                  setStrategyEditorForm({
                    strategyId: selectedStrategy.id,
                    strategyEngine: String(currentParameters.strategyEngine ?? "bk-default"),
                    signalTimeframe: String(
                      currentParameters.signalTimeframe ??
                        selectedStrategy.currentVersion?.signalTimeframe ??
                        "1d"
                    ),
                    executionDataSource: String(
                      currentParameters.executionDataSource ??
                        currentParameters.executionTimeframe ??
                        selectedStrategy.currentVersion?.executionTimeframe ??
                        "tick"
                    ),
                    parametersJson: JSON.stringify(currentParameters, null, 2),
                  });
                }}
              >
                还原当前版本
              </button>
            </div>
          </div>
        </div>

        <div className="live-grid">
          <div className="backtest-list">
            <h4>策略列表</h4>
            <SimpleTable
              columns={["策略", "版本", "信号周期", "执行源", "引擎"]}
              rows={strategies.map((strategy) => {
                const parameters = getRecord(strategy.currentVersion?.parameters);
                return [
                  strategy.name,
                  strategy.currentVersion?.version ?? "--",
                  String(parameters.signalTimeframe ?? strategy.currentVersion?.signalTimeframe ?? "--"),
                  String(parameters.executionDataSource ?? parameters.executionTimeframe ?? strategy.currentVersion?.executionTimeframe ?? "--"),
                  String(parameters.strategyEngine ?? "bk-default"),
                ];
              })}
              emptyMessage="暂无策略"
            />
          </div>
          <div className="backtest-list">
            <h4>当前版本摘要</h4>
            {selectedStrategy ? (
              <div className="backtest-notes">
                <div className="note-item">
                  <strong>策略</strong> {selectedStrategy.name}
                </div>
                <div className="note-item">
                  <strong>说明</strong> {selectedStrategy.description || "--"}
                </div>
                <div className="note-item">
                  <strong>当前版本</strong> {selectedStrategyVersion?.version ?? "--"}
                </div>
                <div className="note-item">
                  <strong>创建时间</strong> {formatTime(selectedStrategy.createdAt)}
                </div>
                <div className="note-item">
                  <strong>引擎</strong> {String(selectedStrategyParameters.strategyEngine ?? "bk-default")}
                </div>
                <div className="note-item">
                  <strong>信号周期</strong> {String(selectedStrategyParameters.signalTimeframe ?? selectedStrategyVersion?.signalTimeframe ?? "--")}
                </div>
                <div className="note-item">
                  <strong>执行源</strong> {String(selectedStrategyParameters.executionDataSource ?? selectedStrategyVersion?.executionTimeframe ?? "--")}
                </div>
                <div className="note-item">
                  <strong>说明</strong> 这一版是直接编辑当前版本参数，不会新建版本。
                </div>
              </div>
            ) : (
              <div className="empty-state empty-state-compact">暂无可编辑策略</div>
            )}
          </div>
        </div>
      </section>
    </div>
  );
}
