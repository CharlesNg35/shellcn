export type CanvasPoint = { x: number; y: number };

export type CanvasRegionShape = "rect" | "circle" | "polygon" | "path";

export interface CanvasRegion {
  id: string;
  shape?: CanvasRegionShape;
  x: number;
  y: number;
  width?: number;
  height?: number;
  radius?: number;
  points?: CanvasPoint[];
  d?: string;
  cursor?: string;
  label?: string;
  capturePointer?: boolean;
}

export interface CanvasFrame {
  commands: CanvasCommand[];
  regions?: CanvasRegion[];
}

export interface CanvasPaint {
  fill?: string | boolean;
  stroke?: string | boolean;
  lineWidth?: number;
  font?: string;
  alpha?: number;
  composite?: GlobalCompositeOperation;
  lineCap?: CanvasLineCap;
  lineJoin?: CanvasLineJoin;
  textAlign?: CanvasTextAlign;
  textBaseline?: CanvasTextBaseline;
  fillId?: string;
  strokeId?: string;
  filter?: string;
  direction?: CanvasDirection;
  miterLimit?: number;
}

export type CanvasGradientKind = "linear" | "radial" | "conic";

export interface CanvasGradientStop {
  offset: number;
  color: string;
}

export type CanvasCommand =
  | ({
      type: "clear";
      color?: string;
      x?: number;
      y?: number;
      width?: number;
      height?: number;
    } & CanvasPaint)
  | {
      type: "set";
      background?: string;
      cursor?: string;
      regions?: CanvasRegion[];
    }
  | { type: "regions"; items?: CanvasRegion[] }
  | { type: "save" }
  | { type: "restore" }
  | { type: "resetTransform" }
  | { type: "translate"; x?: number; y?: number }
  | { type: "scale"; x?: number; y?: number }
  | { type: "rotate"; angle?: number }
  | {
      type: "transform";
      a: number;
      b: number;
      c: number;
      d: number;
      e: number;
      f: number;
    }
  | ({ type: "style" } & CanvasPaint)
  | { type: "resetStyle" }
  | { type: "lineDash"; segments?: number[]; offset?: number }
  | {
      type: "shadow";
      color?: string;
      blur?: number;
      offsetX?: number;
      offsetY?: number;
    }
  | {
      type: "gradient";
      id: string;
      kind: CanvasGradientKind;
      x0?: number;
      y0?: number;
      x1?: number;
      y1?: number;
      r0?: number;
      r1?: number;
      x?: number;
      y?: number;
      startAngle?: number;
      stops?: CanvasGradientStop[];
    }
  | { type: "pattern"; id: string; src: string; repetition?: string }
  | ({
      type: "clip";
      shape?: CanvasRegionShape;
      d?: string;
      x?: number;
      y?: number;
      width?: number;
      height?: number;
      radius?: number;
      points?: CanvasPoint[];
      fillRule?: CanvasFillRule;
    } & CanvasPaint)
  | ({
      type: "rect";
      x?: number;
      y?: number;
      width?: number;
      height?: number;
      radius?: number;
    } & CanvasPaint)
  | ({
      type: "line";
      x1?: number;
      y1?: number;
      x2?: number;
      y2?: number;
    } & CanvasPaint)
  | ({
      type: "arc";
      x?: number;
      y?: number;
      radius?: number;
      startAngle?: number;
      endAngle?: number;
      counterclockwise?: boolean;
    } & CanvasPaint)
  | ({
      type: "quadraticCurve";
      x0?: number;
      y0?: number;
      cpx?: number;
      cpy?: number;
      x?: number;
      y?: number;
    } & CanvasPaint)
  | ({
      type: "bezierCurve";
      x0?: number;
      y0?: number;
      cp1x?: number;
      cp1y?: number;
      cp2x?: number;
      cp2y?: number;
      x?: number;
      y?: number;
    } & CanvasPaint)
  | ({ type: "polyline"; points?: CanvasPoint[] } & CanvasPaint)
  | ({ type: "polygon"; points?: CanvasPoint[] } & CanvasPaint)
  | ({ type: "circle"; x?: number; y?: number; radius?: number } & CanvasPaint)
  | ({
      type: "ellipse";
      x?: number;
      y?: number;
      radiusX?: number;
      radiusY?: number;
      rotation?: number;
    } & CanvasPaint)
  | ({ type: "path"; d?: string; fillRule?: CanvasFillRule } & CanvasPaint)
  | ({
      type: "text";
      x?: number;
      y?: number;
      text?: string;
      maxWidth?: number;
    } & CanvasPaint)
  | ({
      type: "textBox";
      x?: number;
      y?: number;
      width?: number;
      lineHeight?: number;
      text?: string;
    } & CanvasPaint)
  | { type: "measureText"; requestId?: string; text?: string; font?: string }
  | ({
      type: "image";
      src?: string;
      x?: number;
      y?: number;
      width?: number;
      height?: number;
      sourceX?: number;
      sourceY?: number;
      sourceWidth?: number;
      sourceHeight?: number;
      smoothing?: boolean;
    } & CanvasPaint)
  | {
      type: "imageData";
      x?: number;
      y?: number;
      width?: number;
      height?: number;
      data?: number[];
    }
  | {
      type: "snapshot";
      requestId?: string;
      mime?: string;
      quality?: number;
      minIntervalMs?: number;
    };

export type CanvasCommandType = CanvasCommand["type"];

const canvasCommandTypes = [
  "clear",
  "set",
  "regions",
  "save",
  "restore",
  "resetTransform",
  "translate",
  "scale",
  "rotate",
  "transform",
  "style",
  "resetStyle",
  "lineDash",
  "shadow",
  "gradient",
  "pattern",
  "clip",
  "rect",
  "line",
  "arc",
  "quadraticCurve",
  "bezierCurve",
  "polyline",
  "polygon",
  "circle",
  "ellipse",
  "path",
  "text",
  "textBox",
  "measureText",
  "image",
  "imageData",
  "snapshot",
] as const satisfies readonly CanvasCommandType[];

export function isCanvasCommandType(type: string): type is CanvasCommandType {
  return (canvasCommandTypes as readonly string[]).includes(type);
}

export type CanvasOutgoingEvent =
  | {
      type: "ready" | "resize";
      width: number;
      height: number;
      dpr: number;
      theme?: "light" | "dark";
    }
  | {
      type: "pointer";
      event: string;
      x: number;
      y: number;
      button?: number;
      buttons?: number;
      pointerId?: number;
      pointerType?: string;
      regionId?: string;
      modifiers: CanvasModifierState;
    }
  | {
      type: "wheel";
      x: number;
      y: number;
      deltaX: number;
      deltaY: number;
      deltaMode: number;
      modifiers: CanvasModifierState;
    }
  | {
      type: "key";
      event: string;
      key: string;
      code: string;
      repeat: boolean;
      modifiers: CanvasModifierState;
    }
  | {
      type: "textMetrics";
      requestId?: string;
      text?: string;
      width: number;
      [key: string]: string | number | undefined;
    }
  | {
      type: "snapshot";
      requestId?: string;
      mime?: string;
      dataUrl?: string;
      width: number;
      height: number;
    };

export interface CanvasModifierState {
  alt: boolean;
  ctrl: boolean;
  meta: boolean;
  shift: boolean;
}
