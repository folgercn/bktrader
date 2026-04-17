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
import { AccountRecord, ActiveSettingsModal, LiveAccountForm } from "../types/domain";
import {
  ModalActions,
  ModalField,
  ModalFormGrid,
  ModalMetaItem,
  ModalMetaStrip,
  ModalNotice,
  SettingsModalFrame,
} from "./modal-frame";

interface LiveAccountModalProps {
  activeSettingsModal: ActiveSettingsModal;
  setActiveSettingsModal: (modal: ActiveSettingsModal) => void;
  quickLiveAccount: AccountRecord | null;
  liveAccounts: AccountRecord[];
  quickLiveAccountId: string;
  selectQuickLiveAccount: (id: string) => void;
  liveAccountError: string | null;
  liveAccountNotice: string | null;
  liveAccountForm: LiveAccountForm;
  setLiveAccountForm: (valOrUpdater: LiveAccountForm | ((prev: LiveAccountForm) => LiveAccountForm)) => void;
  liveCreateAction: boolean;
  createLiveAccount: () => void;
  openLiveBindingModal: () => void;
}

export function LiveAccountModal({
  activeSettingsModal,
  setActiveSettingsModal,
  quickLiveAccount,
  liveAccounts,
  quickLiveAccountId,
  selectQuickLiveAccount,
  liveAccountError,
  liveAccountNotice,
  liveAccountForm,
  setLiveAccountForm,
  liveCreateAction,
  createLiveAccount,
  openLiveBindingModal,
}: LiveAccountModalProps) {
  const open = activeSettingsModal === "live-account";

  return (
    <SettingsModalFrame
      open={open}
      onOpenChange={(nextOpen) => !nextOpen && setActiveSettingsModal(null)}
      kicker="Live Account"
      title="新建实盘 / Testnet 账户"
      description="保留当前账户上下文，在这里创建或切换账户，再继续进入适配器绑定。"
      className="max-w-[min(720px,calc(100vw-2rem))]"
    >
      <ModalMetaStrip>
        <ModalMetaItem
          label="Current"
          value={`${quickLiveAccount?.name ?? "--"} · ${quickLiveAccount?.status ?? "--"} · ${quickLiveAccount?.exchange ?? "--"}`}
        />
        <ModalMetaItem
          label="Accounts"
          value={liveAccounts.length > 0 ? liveAccounts.map((item) => item.name).join(" / ") : "暂无账户"}
        />
      </ModalMetaStrip>

      {liveAccountError ? <ModalNotice tone="error">{liveAccountError}</ModalNotice> : null}
      {liveAccountNotice ? <ModalNotice tone="success">{liveAccountNotice}</ModalNotice> : null}

      <ModalFormGrid>
        {liveAccounts.length > 0 ? (
          <ModalField label="切换到已有账户" wide>
            <Select value={quickLiveAccountId} onValueChange={(value) => value && selectQuickLiveAccount(value)}>
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
        ) : null}

        <ModalField label="Name">
          <Input
            value={liveAccountForm.name}
            onChange={(event) => setLiveAccountForm((current) => ({ ...current, name: event.target.value }))}
            className="h-10 rounded-xl border-[var(--bk-border)] bg-[var(--bk-surface-overlay)] px-3"
          />
        </ModalField>
        <ModalField label="Exchange">
          <Input
            value={liveAccountForm.exchange}
            onChange={(event) => setLiveAccountForm((current) => ({ ...current, exchange: event.target.value }))}
            className="h-10 rounded-xl border-[var(--bk-border)] bg-[var(--bk-surface-overlay)] px-3"
          />
        </ModalField>
      </ModalFormGrid>

      <ModalNotice tone="info">
        默认会自动补一个不冲突的 testnet 名称，避免和已有账户重名。
      </ModalNotice>

      <ModalActions>
        <Button
          variant="bento-outline"
          className="h-10 rounded-xl px-4 font-bold"
          disabled={!quickLiveAccountId}
          onClick={() => {
            if (quickLiveAccountId) {
              selectQuickLiveAccount(quickLiveAccountId);
            }
            openLiveBindingModal();
          }}
        >
          使用当前选中账户去绑定
        </Button>
        <Button
          variant="bento"
          className="h-10 rounded-xl px-5 font-black"
          disabled={liveCreateAction || !liveAccountForm.name.trim() || !liveAccountForm.exchange.trim()}
          onClick={createLiveAccount}
        >
          {liveCreateAction ? "Creating..." : "Create Live Account"}
        </Button>
      </ModalActions>
    </SettingsModalFrame>
  );
}
