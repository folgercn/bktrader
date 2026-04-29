import React, { useState, useEffect } from 'react';
import { SettingsModalFrame } from './modal-frame';
import { Order, Fill, RemoteFillsResponse, ManualFillSyncResponse } from '../types/domain';
import { fetchRemoteFills, manualSyncFills } from '../utils/api';
import { Button } from '../components/ui/button';
import { Input } from '../components/ui/input';
import { AlertCircle, CheckCircle2, ChevronDown, ChevronRight, RefreshCw, Server, Info } from 'lucide-react';

interface FillSyncModalProps {
  isOpen: boolean;
  onClose: () => void;
  order: Order | null;
  onSuccess?: () => void;
  initialMode?: 'view' | 'sync';
}

export function FillSyncModal({ isOpen, onClose, order, onSuccess, initialMode = 'view' }: FillSyncModalProps) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [remoteData, setRemoteData] = useState<RemoteFillsResponse | null>(null);
  const [syncResult, setSyncResult] = useState<ManualFillSyncResponse | null>(null);
  
  const [mode, setMode] = useState<'view' | 'sync'>(initialMode);
  const [reason, setReason] = useState('');
  const [syncing, setSyncing] = useState(false);
  const [showRaw, setShowRaw] = useState(false);

  useEffect(() => {
    if (isOpen && order && order.accountId) {
      loadRemoteFills();
      setMode(initialMode);
      setSyncResult(null);
      setReason('');
      setShowRaw(false);
    }
  }, [isOpen, order, initialMode]);

  const loadRemoteFills = async () => {
    if (!order) return;
    setLoading(true);
    setError(null);
    try {
      const data = await fetchRemoteFills(order.id);
      setRemoteData(data);
    } catch (err: any) {
      setError(err.message || 'Failed to fetch remote fills');
    } finally {
      setLoading(false);
    }
  };

  const handleSync = async (dryRun: boolean) => {
    if (!order) return;
    if (!dryRun && !reason.trim()) {
      setError('Reason is required for manual sync');
      return;
    }
    setSyncing(true);
    setError(null);
    try {
      const result = await manualSyncFills(order.id, {
        confirm: true,
        reason: reason.trim(),
        dryRun
      });
      setSyncResult(result);
      if (!dryRun && result.result === 'settled' && onSuccess) {
        onSuccess();
      }
    } catch (err: any) {
      setError(err.message || 'Manual sync failed');
    } finally {
      setSyncing(false);
    }
  };

  if (!order) return null;

  return (
    <SettingsModalFrame open={isOpen} onOpenChange={(o) => !o && onClose()} kicker="Diagnostics" title={`Fill Sync Diagnostics: ${order.id}`}>
      <div className="flex flex-col h-full overflow-hidden">
        
        <div className="flex border-b border-border mb-4">
          <button
            className={`px-4 py-2 text-sm font-medium border-b-2 ${mode === 'view' ? 'border-primary text-primary' : 'border-transparent text-muted-foreground hover:text-foreground'}`}
            onClick={() => setMode('view')}
          >
            Remote Details
          </button>
          <button
            className={`px-4 py-2 text-sm font-medium border-b-2 ${mode === 'sync' ? 'border-primary text-primary' : 'border-transparent text-muted-foreground hover:text-foreground'}`}
            onClick={() => setMode('sync')}
          >
            Manual Sync
          </button>
        </div>

        <div className="flex-1 overflow-y-auto pr-2 pb-4 space-y-6">
          {error && (
            <div className="rounded-2xl border border-[var(--bk-status-danger)]/25 bg-[color:color-mix(in_srgb,var(--bk-status-danger)_8%,transparent)] p-4 text-[12px] font-bold text-[var(--bk-status-danger)] flex items-start">
              <AlertCircle className="h-4 w-4 mr-2 mt-0.5 shrink-0" />
              <div>
                <h4 className="font-black">Error</h4>
                <div className="font-normal">{error}</div>
              </div>
            </div>
          )}

          {loading ? (
            <div className="flex items-center justify-center p-8 text-muted-foreground">
              <RefreshCw className="h-6 w-6 animate-spin mr-2" />
              Loading remote fills from exchange...
            </div>
          ) : mode === 'view' && remoteData ? (
            <div className="space-y-6">
              <div className="grid grid-cols-2 gap-4">
                <div className="bg-muted/30 p-4 rounded-lg border border-border">
                  <h3 className="font-medium mb-2 flex items-center"><Server className="w-4 h-4 mr-2" />Remote Order</h3>
                  <div className="text-sm space-y-1">
                    <div className="flex justify-between"><span className="text-muted-foreground">Status:</span> <span>{String(remoteData.remoteOrder?.status || 'Unknown')}</span></div>
                    <div className="flex justify-between"><span className="text-muted-foreground">Exchange ID:</span> <span className="font-mono text-xs truncate max-w-[150px]">{remoteData.exchangeOrderId}</span></div>
                    <div className="flex justify-between"><span className="text-muted-foreground">Filled Qty:</span> <span>{String(remoteData.remoteOrder?.filledQuantity || 0)}</span></div>
                  </div>
                </div>
                
                <div className="bg-muted/30 p-4 rounded-lg border border-border">
                  <h3 className="font-medium mb-2 flex items-center"><Info className="w-4 h-4 mr-2" />Diagnostics</h3>
                  <div className="text-sm space-y-1">
                    <div className="flex justify-between"><span className="text-muted-foreground">Remote Trades:</span> <span>{remoteData.diagnostics.remoteTradeCount}</span></div>
                    <div className="flex justify-between"><span className="text-muted-foreground">Local Fills:</span> <span>{remoteData.diagnostics.localFillCount} ({remoteData.diagnostics.localRealFillCount} real, {remoteData.diagnostics.localSyntheticFillCount} syn)</span></div>
                    <div className="flex justify-between"><span className="text-muted-foreground">Can Settle:</span> <span className={remoteData.diagnostics.canSettle ? 'text-green-500 font-medium' : ''}>{remoteData.diagnostics.canSettle ? 'Yes' : 'No'}</span></div>
                  </div>
                  <div className="text-xs text-muted-foreground mt-2 border-t border-border pt-2">
                    {remoteData.diagnostics.reason}
                  </div>
                </div>
              </div>

              <div>
                <h3 className="font-medium mb-3">Normalized Remote Trades ({remoteData.normalizedReports.length})</h3>
                {remoteData.normalizedReports.length > 0 ? (
                  <div className="border border-border rounded-md overflow-hidden text-sm">
                    <table className="w-full">
                      <thead className="bg-muted/50 border-b border-border">
                        <tr>
                          <th className="text-left px-3 py-2 font-medium">Price</th>
                          <th className="text-left px-3 py-2 font-medium">Quantity</th>
                          <th className="text-left px-3 py-2 font-medium">Fee</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-border">
                        {remoteData.normalizedReports.map((r: any, i) => (
                          <tr key={i} className="bg-background">
                            <td className="px-3 py-2">{r.price}</td>
                            <td className="px-3 py-2">{r.quantity}</td>
                            <td className="px-3 py-2">{r.fee}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                ) : (
                  <div className="text-sm text-muted-foreground p-4 bg-muted/20 rounded-md border border-border border-dashed text-center">
                    No remote trades found for this order.
                  </div>
                )}
              </div>

              <div>
                <button 
                  onClick={() => setShowRaw(!showRaw)}
                  className="flex items-center text-sm font-medium text-muted-foreground hover:text-foreground"
                >
                  {showRaw ? <ChevronDown className="w-4 h-4 mr-1" /> : <ChevronRight className="w-4 h-4 mr-1" />}
                  Raw Exchange Response (Masked)
                </button>
                {showRaw && remoteData.raw && (
                  <pre className="mt-2 p-4 bg-muted rounded-md text-xs font-mono overflow-x-auto border border-border">
                    {JSON.stringify(remoteData.raw, null, 2)}
                  </pre>
                )}
              </div>
            </div>
          ) : mode === 'sync' && (
            <div className="space-y-6">
              <div className="rounded-2xl border border-blue-500/25 bg-blue-500/10 p-4 text-[12px] text-blue-500/90 flex items-start">
                <AlertCircle className="h-4 w-4 mr-2 mt-0.5 shrink-0" />
                <div>
                  <h4 className="font-black">Manual Synchronization</h4>
                  <div className="font-normal mt-1">
                    This operation will re-fetch exchange trades and trigger the existing fill reconciliation pipeline to update local fills, order metadata, and positions. 
                    <br className="my-1"/>
                    <strong>It will not place or cancel orders.</strong>
                  </div>
                </div>
              </div>

              <div className="space-y-3">
                <label className="text-sm font-medium">Reason for Manual Sync <span className="text-red-500">*</span></label>
                <Input 
                  value={reason} 
                  onChange={(e) => setReason(e.target.value)} 
                  placeholder="E.g., Replacing synthetic fills with real exchange trades due to WS drop"
                  disabled={syncing || (syncResult != null && !syncResult.dryRun)}
                />
              </div>

              {syncResult && (
                <div className={`p-4 rounded-lg border ${syncResult.dryRun ? 'bg-blue-500/10 border-blue-500/20' : (syncResult.result === 'settled' ? 'bg-green-500/10 border-green-500/20' : 'bg-muted/50 border-border')}`}>
                  <h3 className="font-medium mb-3 flex items-center">
                    {syncResult.dryRun ? <Info className="w-4 h-4 mr-2 text-blue-500" /> : <CheckCircle2 className="w-4 h-4 mr-2 text-green-500" />}
                    {syncResult.dryRun ? 'Dry Run Results' : 'Sync Completed'}
                    <span className="ml-auto text-xs px-2 py-1 bg-background rounded-full border border-border">
                      {syncResult.result}
                    </span>
                  </h3>
                  
                  {syncResult.changes && (
                    <div className="mb-4 grid grid-cols-2 gap-2 text-xs">
                      <div className="flex justify-between items-center p-2 rounded bg-background/50 border border-border/50">
                        <span className="text-muted-foreground">Deleted Synthetic:</span>
                        <span className="font-bold text-orange-500">{syncResult.changes.deletedSyntheticCount}</span>
                      </div>
                      <div className="flex justify-between items-center p-2 rounded bg-background/50 border border-border/50">
                        <span className="text-muted-foreground">Added Real:</span>
                        <span className="font-bold text-green-500">{syncResult.changes.addedRealCount}</span>
                      </div>
                      {syncResult.changes.duplicateTradeIDs && syncResult.changes.duplicateTradeIDs.length > 0 && (
                        <div className="col-span-2 p-2 rounded bg-background/50 border border-border/50 text-muted-foreground">
                          Skipped {syncResult.changes.duplicateTradeIDs.length} duplicates.
                        </div>
                      )}
                    </div>
                  )}

                  <div className="grid grid-cols-2 gap-4 text-sm mt-4 border-t border-border/30 pt-4">
                    <div className="space-y-2">
                      <div className="font-medium text-muted-foreground mb-1 border-b border-border pb-1">Before</div>
                      <div className="flex justify-between"><span>Total Fills:</span> <span>{syncResult.before.fillCount}</span></div>
                      <div className="flex justify-between"><span>Real:</span> <span>{syncResult.before.realFillCount}</span></div>
                      <div className="flex justify-between"><span>Synthetic:</span> <span>{syncResult.before.syntheticFillCount}</span></div>
                      <div className="flex justify-between"><span>Filled Qty:</span> <span>{syncResult.before.filledQuantity}</span></div>
                    </div>
                    <div className="space-y-2">
                      <div className="font-medium text-muted-foreground mb-1 border-b border-border pb-1">{syncResult.dryRun ? 'After (Predicted)' : 'After'}</div>
                      <div className="flex justify-between"><span>Total Fills:</span> <span>{syncResult.after.fillCount}</span></div>
                      <div className="flex justify-between"><span>Real:</span> <span className={syncResult.after.realFillCount > syncResult.before.realFillCount ? 'text-green-500 font-medium' : ''}>{syncResult.after.realFillCount}</span></div>
                      <div className="flex justify-between"><span>Synthetic:</span> <span className={syncResult.after.syntheticFillCount < syncResult.before.syntheticFillCount ? 'text-blue-500 font-medium' : ''}>{syncResult.after.syntheticFillCount}</span></div>
                      <div className="flex justify-between"><span>Filled Qty:</span> <span className={syncResult.after.filledQuantity !== syncResult.before.filledQuantity ? 'text-blue-500 font-medium' : ''}>{syncResult.after.filledQuantity}</span></div>
                    </div>
                  </div>
                </div>
              )}

              <div className="flex justify-end space-x-3 pt-4 border-t border-border">
                <Button 
                  variant="outline" 
                  onClick={() => handleSync(true)} 
                  disabled={syncing || (syncResult != null && !syncResult.dryRun)}
                >
                  {syncing && syncResult?.dryRun !== false ? <RefreshCw className="mr-2 h-4 w-4 animate-spin" /> : null}
                  Preview (Dry Run)
                </Button>
                <Button 
                  variant="default" 
                  onClick={() => handleSync(false)} 
                  disabled={syncing || !reason.trim() || (syncResult != null && !syncResult.dryRun)}
                >
                  {syncing && syncResult?.dryRun === false ? <RefreshCw className="mr-2 h-4 w-4 animate-spin" /> : null}
                  Confirm Sync
                </Button>
              </div>
            </div>
          )}
        </div>
      </div>
    </SettingsModalFrame>
  );
}
