import React from "react";
import { useUIStore } from "../store/useUIStore";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "../components/ui/alert-dialog";

export function ConfirmModal() {
  const confirmDialogConfig = useUIStore((s) => s.confirmDialogConfig);
  const closeConfirmDialog = useUIStore((s) => s.closeConfirmDialog);

  return (
    <AlertDialog open={confirmDialogConfig.isOpen} onOpenChange={(open) => !open && closeConfirmDialog()}>
      <AlertDialogContent tone="bento" className="rounded-[32px] border-2 border-[var(--bk-border-strong)] p-8">
        <AlertDialogHeader>
          <AlertDialogTitle className="text-xl font-black text-[var(--bk-text-primary)]">{confirmDialogConfig.title}</AlertDialogTitle>
          <AlertDialogDescription className="font-medium leading-relaxed text-[var(--bk-text-muted)]">
            {confirmDialogConfig.description}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter className="gap-3">
          <AlertDialogCancel 
            onClick={closeConfirmDialog}
            variant="bento-outline"
            className="rounded-xl border-2 font-bold transition-all"
          >
            取消
          </AlertDialogCancel>
          <AlertDialogAction
            variant="bento-danger"
            className="rounded-xl px-6 py-2 font-black shadow-lg transition-all active:scale-95"
            onClick={() => {
              confirmDialogConfig.onConfirm();
              closeConfirmDialog();
            }}
          >
            确认强制执行
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
