import { useConfirm } from "primevue/useconfirm";

interface ConfirmDangerOptions {
  header: string;
  message: string;
  acceptLabel?: string;
  accept: () => void | Promise<void>;
}

// Standard destructive confirmation: a red accept button + a neutral Cancel,
// routed through PrimeVue's ConfirmationService (the global <ConfirmDialog>).
export function useConfirmAction() {
  let confirm: ReturnType<typeof useConfirm> | null = null;
  try {
    confirm = useConfirm();
  } catch {
    confirm = null;
  }

  function confirmDanger(options: ConfirmDangerOptions): void {
    if (!confirm) {
      if (window.confirm(options.message)) void options.accept();
      return;
    }
    confirm.require({
      header: options.header,
      message: options.message,
      rejectProps: { label: "Cancel", severity: "secondary", text: true },
      acceptProps: {
        label: options.acceptLabel ?? "Delete",
        severity: "danger",
      },
      accept: options.accept,
    });
  }

  return { confirmDanger };
}
