import React from "react";

import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../components/ui/select";
import { AccountRecord, ActiveSettingsModal, LiveAdapter, LiveBindingForm } from "../types/domain";
import {
  ModalActions,
  ModalCheckboxField,
  ModalField,
  ModalFormGrid,
  ModalMetaItem,
  ModalMetaStrip,
  ModalNotice,
  SettingsModalFrame,
} from "./modal-frame";

interface LiveBindingModalProps {
  activeSettingsModal: ActiveSettingsModal;
  setActiveSettingsModal: (modal: ActiveSettingsModal) => void;
  liveBindingError: string | null;
  liveBindingNotice: string | null;
  liveBindingForm: LiveBindingForm;
  setLiveBindingForm: (valOrUpdater: LiveBindingForm | ((prev: LiveBindingForm) => LiveBindingForm)) => void;
  liveAccounts: AccountRecord[];
  liveAdapters: LiveAdapter[];
  quickLiveAccount: AccountRecord | null;
  liveBindAction: boolean;
  bindLiveAccount: () => void;
}

export function LiveBindingModal({
  activeSettingsModal,
  setActiveSettingsModal,
  liveBindingError,
  liveBindingNotice,
  liveBindingForm,
  setLiveBindingForm,
  liveAccounts,
  liveAdapters,
  quickLiveAccount,
  liveBindAction,
  bindLiveAccount,
}: LiveBindingModalProps) {
  const open = activeSettingsModal === "live-binding";

  return (
    <SettingsModalFrame
      open={open}
      onOpenChange={(nextOpen) => !nextOpen && setActiveSettingsModal(null)}
      kicker="Live Binding"
      title="绑定 Live / Testnet 适配器"
      description="账户、适配器和 sandbox 行为在这里收敛，保留原有实盘控制边界。"
      className="max-w-[min(820px,calc(100vw-2rem))]"
    >
      {liveBindingError ? <ModalNotice tone="error">{liveBindingError}</ModalNotice> : null}
      {liveBindingNotice ? <ModalNotice tone="success">{liveBindingNotice}</ModalNotice> : null}

      <ModalMetaStrip>
        <ModalMetaItem
          label="Binding"
          value={`${String(quickLiveAccount?.bindings?.live?.adapterKey ?? "--")} · sandbox ${String(quickLiveAccount?.bindings?.live?.sandbox ?? "--")}`}
        />
        <ModalMetaItem label="Adapters" value={String(liveAdapters.length)} />
      </ModalMetaStrip>

      <ModalFormGrid>
        <ModalField label="Live Account">
          <Select
            value={liveBindingForm.accountId}
            onValueChange={(value) => setLiveBindingForm((current) => ({ ...current, accountId: value ?? "" }))}
          >
            <SelectTrigger tone="bento" className="h-10 w-full rounded-xl">
              <SelectValue placeholder="选择账户" />
            </SelectTrigger>
            <SelectContent tone="bento" className="rounded-xl">
              {liveAccounts.map((account) => (
                <SelectItem key={account.id} value={account.id}>
                  {account.name} ({account.status})
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </ModalField>

        <ModalField label="Adapter">
          <Select
            value={liveBindingForm.adapterKey}
            onValueChange={(value) => setLiveBindingForm((current) => ({ ...current, adapterKey: value ?? "" }))}
          >
            <SelectTrigger tone="bento" className="h-10 w-full rounded-xl">
              <SelectValue placeholder="选择适配器" />
            </SelectTrigger>
            <SelectContent tone="bento" className="rounded-xl">
              {liveAdapters.map((adapter) => (
                <SelectItem key={adapter.key} value={adapter.key}>
                  {adapter.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </ModalField>

        <ModalField label="Position Mode">
          <Select
            value={liveBindingForm.positionMode}
            onValueChange={(value) => setLiveBindingForm((current) => ({ ...current, positionMode: value ?? "" }))}
          >
            <SelectTrigger tone="bento" className="h-10 w-full rounded-xl">
              <SelectValue placeholder="Position Mode" />
            </SelectTrigger>
            <SelectContent tone="bento" className="rounded-xl">
              <SelectItem value="ONE_WAY">ONE_WAY</SelectItem>
              <SelectItem value="HEDGE">HEDGE</SelectItem>
            </SelectContent>
          </Select>
        </ModalField>

        <ModalField label="Margin Mode">
          <Select
            value={liveBindingForm.marginMode}
            onValueChange={(value) => setLiveBindingForm((current) => ({ ...current, marginMode: value ?? "" }))}
          >
            <SelectTrigger tone="bento" className="h-10 w-full rounded-xl">
              <SelectValue placeholder="Margin Mode" />
            </SelectTrigger>
            <SelectContent tone="bento" className="rounded-xl">
              <SelectItem value="CROSSED">CROSSED</SelectItem>
              <SelectItem value="ISOLATED">ISOLATED</SelectItem>
            </SelectContent>
          </Select>
        </ModalField>

        <ModalField label="API Key Env">
          <Input
            value={liveBindingForm.apiKeyRef}
            onChange={(event) => setLiveBindingForm((current) => ({ ...current, apiKeyRef: event.target.value }))}
            className="h-10 rounded-xl border-[var(--bk-border)] bg-[var(--bk-surface-overlay)] px-3"
          />
        </ModalField>

        <ModalField label="API Secret Env">
          <Input
            value={liveBindingForm.apiSecretRef}
            onChange={(event) => setLiveBindingForm((current) => ({ ...current, apiSecretRef: event.target.value }))}
            className="h-10 rounded-xl border-[var(--bk-border)] bg-[var(--bk-surface-overlay)] px-3"
          />
        </ModalField>

        <div className="md:col-span-2">
          <ModalCheckboxField
            label="Sandbox"
            checked={liveBindingForm.sandbox}
            onChange={(checked) => setLiveBindingForm((current) => ({ ...current, sandbox: checked }))}
          />
        </div>
      </ModalFormGrid>

      <ModalNotice tone="info">
        sandbox=true 时默认从 <code>.env</code> 读取 <code>BINANCE_TESTNET_API_KEY</code> / <code>BINANCE_TESTNET_API_SECRET</code>。
      </ModalNotice>

      <ModalActions>
        <Button
          variant="bento"
          className="h-10 rounded-xl px-5 font-black"
          disabled={liveBindAction || !liveBindingForm.accountId}
          onClick={bindLiveAccount}
        >
          {liveBindAction ? "Binding..." : "Bind Live Adapter"}
        </Button>
      </ModalActions>
    </SettingsModalFrame>
  );
}
