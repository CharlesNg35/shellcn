import { useToast } from "primevue/usetoast";

export function useNotify() {
  const toast = useToast();
  return {
    success(summary: string, detail?: string): void {
      toast.add({ severity: "success", summary, detail, life: 3000 });
    },
    error(summary: string, detail?: string): void {
      toast.add({ severity: "error", summary, detail, life: 5000 });
    },
    info(summary: string, detail?: string): void {
      toast.add({ severity: "info", summary, detail, life: 3000 });
    },
  };
}
