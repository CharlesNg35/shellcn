import { useConfirm } from "primevue/useconfirm";

interface DirtyGuardOptions {
  isDirty: () => boolean;
  header?: string;
  message?: string;
  acceptLabel?: string;
}

export function useDirtyGuard(options: DirtyGuardOptions) {
  let confirm: ReturnType<typeof useConfirm> | null = null;
  try {
    confirm = useConfirm();
  } catch {
    confirm = null;
  }

  function confirmBeforeDiscard(
    action: () => void | Promise<void>,
  ): Promise<boolean> {
    if (!options.isDirty()) {
      return Promise.resolve(runAction(action));
    }

    return new Promise((resolve) => {
      const accept = async (): Promise<void> => {
        resolve(await runAction(action));
      };
      const reject = (): void => resolve(false);
      const message =
        options.message ??
        "You have unsaved changes. Discard them and continue?";

      if (!confirm) {
        if (!window.confirm(message)) {
          reject();
          return;
        }
        void accept();
        return;
      }

      confirm.require({
        header: options.header ?? "Discard unsaved changes?",
        message,
        rejectProps: {
          label: "Keep editing",
          severity: "secondary",
          text: true,
        },
        acceptProps: {
          label: options.acceptLabel ?? "Discard changes",
          severity: "warn",
        },
        accept,
        reject,
      });
    });
  }

  async function runAction(
    action: () => void | Promise<void>,
  ): Promise<boolean> {
    await action();
    return true;
  }

  return { confirmBeforeDiscard };
}
