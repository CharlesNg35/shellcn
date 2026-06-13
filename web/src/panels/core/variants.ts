import { isVisible } from "@/panels/form/condition";
import type { PanelType, PanelVariant } from "@/types/projection";

export interface VariantPanel {
  panel: PanelType;
  config?: Record<string, unknown>;
  variants?: PanelVariant[];
}

export function activePanelVariant(
  panel: VariantPanel,
  data: Record<string, unknown>,
): PanelVariant | undefined {
  return panel.variants?.find((variant) =>
    isVisible(variant.visibleWhen, data),
  );
}

export function resolvedPanelType(
  panel: VariantPanel,
  data: Record<string, unknown>,
): PanelType {
  return activePanelVariant(panel, data)?.panel ?? panel.panel;
}

export function resolvedPanelConfig(
  panel: VariantPanel,
  data: Record<string, unknown>,
): Record<string, unknown> {
  return activePanelVariant(panel, data)?.config ?? panel.config ?? {};
}
