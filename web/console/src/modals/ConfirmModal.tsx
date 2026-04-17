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
      <AlertDialogContent className="border-2 border-[#d8cfba] rounded-[32px] bg-[#fffbf2]">
        <AlertDialogHeader>
          <AlertDialogTitle className="text-xl font-black text-[#1f2328]">{confirmDialogConfig.title}</AlertDialogTitle>
          <AlertDialogDescription className="text-[#687177] font-medium leading-relaxed">
            {confirmDialogConfig.description}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter className="gap-3">
          <AlertDialogCancel 
            onClick={closeConfirmDialog}
            className="rounded-xl border-2 border-[#d8cfba] font-bold text-[#687177] hover:bg-[#ebe5d5] transition-all"
          >
            取消
          </AlertDialogCancel>
          <AlertDialogAction
            className="bg-rose-600 hover:bg-rose-700 text-white font-black rounded-xl px-6 py-2 shadow-lg transition-all active:scale-95"
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
