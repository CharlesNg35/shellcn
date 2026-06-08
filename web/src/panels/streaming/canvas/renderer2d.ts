import type {
  CanvasCommand,
  CanvasFrame,
  CanvasPaint,
  CanvasPoint,
  CanvasRegion,
  CanvasOutgoingEvent,
} from "./types";

type EmitEvent = (event: CanvasOutgoingEvent) => void;
const MAX_PATH_CACHE = 256;
const MAX_TEXT_WRAP_CACHE = 256;

export class Canvas2DRenderer {
  private canvas: HTMLCanvasElement | null = null;
  private ctx: CanvasRenderingContext2D | null = null;
  private readonly emit: EmitEvent;
  private imageCache = new Map<string, HTMLImageElement>();
  private resources = new Map<string, CanvasGradient | CanvasPattern>();
  private snapshotTimes = new Map<string, number>();
  private pathCache = new Map<string, Path2D | null>();
  private textWrapCache = new Map<string, string[]>();
  private regions: CanvasRegion[] = [];
  private lastFrame: CanvasFrame | null = null;
  private lastBackground: string | undefined;
  private frameVersion = 0;
  private width = 800;
  private height = 450;
  private dpr = 1;

  constructor(emit: EmitEvent) {
    this.emit = emit;
  }

  attach(canvas: HTMLCanvasElement): void {
    this.canvas = canvas;
    this.ctx = canvas.getContext("2d");
  }

  resize(
    parent: HTMLElement,
    background?: string,
    hidpi = true,
    content?: { width?: number; height?: number },
  ): { width: number; height: number; dpr: number } {
    if (!this.canvas) return this.size();
    const rect = parent.getBoundingClientRect();
    this.width = Math.max(
      1,
      Math.round(Math.max(rect.width || this.width, content?.width || 0)),
    );
    this.height = Math.max(
      1,
      Math.round(Math.max(rect.height || this.height, content?.height || 0)),
    );
    this.dpr = hidpi ? window.devicePixelRatio || 1 : 1;
    const backingWidth = Math.round(this.width * this.dpr);
    const backingHeight = Math.round(this.height * this.dpr);
    const styleWidth = `${this.width}px`;
    const styleHeight = `${this.height}px`;
    if (
      this.ctx &&
      this.canvas.width === backingWidth &&
      this.canvas.height === backingHeight &&
      this.canvas.style.width === styleWidth &&
      this.canvas.style.height === styleHeight
    ) {
      return this.size();
    }
    this.canvas.width = backingWidth;
    this.canvas.height = backingHeight;
    this.canvas.style.width = styleWidth;
    this.canvas.style.height = styleHeight;
    this.ctx = this.canvas.getContext("2d");
    this.ctx?.setTransform(this.dpr, 0, 0, this.dpr, 0, 0);
    this.clear(background);
    return this.size();
  }

  size(): { width: number; height: number; dpr: number } {
    return { width: this.width, height: this.height, dpr: this.dpr };
  }

  currentRegions(): CanvasRegion[] {
    return this.regions;
  }

  render(frame: CanvasFrame, background?: string): void {
    this.lastFrame = frame;
    this.lastBackground = background;
    this.frameVersion++;
    this.paint(frame, background, this.frameVersion);
  }

  private paint(
    frame: CanvasFrame,
    background: string | undefined,
    version: number,
  ): void {
    this.resetFrameState();
    for (const command of frame.commands)
      this.run(command, background, version);
    if (frame.regions) this.regions = frame.regions;
  }

  pointFromEvent(ev: MouseEvent | WheelEvent): CanvasPoint {
    if (!this.canvas) return { x: 0, y: 0 };
    const rect = this.canvas.getBoundingClientRect();
    return {
      x: ((ev.clientX - rect.left) / Math.max(1, rect.width)) * this.width,
      y: ((ev.clientY - rect.top) / Math.max(1, rect.height)) * this.height,
    };
  }

  regionAt(point: CanvasPoint): CanvasRegion | undefined {
    for (let index = this.regions.length - 1; index >= 0; index--) {
      const region = this.regions[index];
      if (this.regionContains(region, point)) return region;
    }
    return undefined;
  }

  private run(
    command: CanvasCommand,
    background: string | undefined,
    version: number,
  ): void {
    const ctx = this.ctx;
    if (!ctx) return;
    switch (command.type) {
      case "clear":
        this.clear(
          typeof command.color === "string" ? command.color : background,
          command,
        );
        break;
      case "set":
        if (typeof command.background === "string")
          this.clear(command.background);
        if (Array.isArray(command.regions)) this.regions = command.regions;
        if (typeof command.cursor === "string" && this.canvas)
          this.canvas.style.cursor = command.cursor;
        break;
      case "regions":
        this.regions = Array.isArray(command.items) ? command.items : [];
        break;
      case "save":
        ctx.save();
        break;
      case "restore":
        ctx.restore();
        break;
      case "resetTransform":
        ctx.setTransform(this.dpr, 0, 0, this.dpr, 0, 0);
        break;
      case "translate":
        ctx.translate(num(command.x), num(command.y));
        break;
      case "scale":
        ctx.scale(num(command.x, 1), num(command.y, num(command.x, 1)));
        break;
      case "rotate":
        ctx.rotate(num(command.angle));
        break;
      case "transform":
        ctx.transform(
          command.a,
          command.b,
          command.c,
          command.d,
          command.e,
          command.f,
        );
        break;
      case "style":
        this.applyStyle(command);
        break;
      case "resetStyle":
        this.resetStyle();
        break;
      case "lineDash":
        ctx.setLineDash(command.segments ?? []);
        ctx.lineDashOffset = num(command.offset);
        break;
      case "shadow":
        ctx.shadowColor = command.color ?? "transparent";
        ctx.shadowBlur = num(command.blur);
        ctx.shadowOffsetX = num(command.offsetX);
        ctx.shadowOffsetY = num(command.offsetY);
        break;
      case "gradient":
        this.defineGradient(command);
        break;
      case "pattern":
        this.definePattern(command, version);
        break;
      case "clip":
        this.clip(command);
        break;
      case "rect":
        this.drawRect(command);
        break;
      case "line":
        this.drawLine(command);
        break;
      case "arc":
        this.drawArc(command);
        break;
      case "quadraticCurve":
        this.drawQuadratic(command);
        break;
      case "bezierCurve":
        this.drawBezier(command);
        break;
      case "polyline":
      case "polygon":
        this.drawPolyline(command, command.type === "polygon");
        break;
      case "circle":
        this.drawCircle(command);
        break;
      case "ellipse":
        this.drawEllipse(command);
        break;
      case "path":
        this.drawPath(command);
        break;
      case "text":
        this.drawText(command);
        break;
      case "textBox":
        this.drawTextBox(command);
        break;
      case "measureText":
        this.measureText(command);
        break;
      case "image":
        this.drawImage(command, version);
        break;
      case "imageData":
        this.drawImageData(command);
        break;
      case "snapshot":
        this.snapshot(command);
        break;
    }
  }

  private clear(
    color?: string,
    rect?: { x?: number; y?: number; width?: number; height?: number },
  ): void {
    const ctx = this.ctx;
    if (!ctx) return;
    const x = num(rect?.x);
    const y = num(rect?.y);
    const width = num(rect?.width, this.width);
    const height = num(rect?.height, this.height);
    ctx.save();
    ctx.setTransform(this.dpr, 0, 0, this.dpr, 0, 0);
    ctx.globalAlpha = 1;
    ctx.globalCompositeOperation = "source-over";
    ctx.filter = "none";
    ctx.clearRect(x, y, width, height);
    if (color) {
      ctx.fillStyle = color;
      ctx.fillRect(x, y, width, height);
    }
    ctx.restore();
  }

  private resetFrameState(): void {
    const ctx = this.ctx;
    if (!ctx) return;
    ctx.setTransform(this.dpr, 0, 0, this.dpr, 0, 0);
    this.resetStyle();
  }

  private resetStyle(): void {
    const ctx = this.ctx;
    if (!ctx) return;
    ctx.globalAlpha = 1;
    ctx.globalCompositeOperation = "source-over";
    ctx.fillStyle = "#000000";
    ctx.strokeStyle = "#000000";
    ctx.lineWidth = 1;
    ctx.lineCap = "butt";
    ctx.lineJoin = "miter";
    ctx.miterLimit = 10;
    ctx.setLineDash([]);
    ctx.lineDashOffset = 0;
    ctx.shadowColor = "transparent";
    ctx.shadowBlur = 0;
    ctx.shadowOffsetX = 0;
    ctx.shadowOffsetY = 0;
    ctx.filter = "none";
    ctx.direction = "inherit";
    ctx.font = "10px sans-serif";
    ctx.textAlign = "start";
    ctx.textBaseline = "alphabetic";
  }

  private applyStyle(command: CanvasPaint): void {
    const ctx = this.ctx;
    if (!ctx) return;
    const fill = command.fillId
      ? this.resources.get(command.fillId)
      : command.fill;
    const stroke = command.strokeId
      ? this.resources.get(command.strokeId)
      : command.stroke;
    if (typeof fill === "string" || isCanvasStyle(fill)) ctx.fillStyle = fill;
    if (typeof stroke === "string" || isCanvasStyle(stroke))
      ctx.strokeStyle = stroke;
    if (typeof command.lineWidth === "number")
      ctx.lineWidth = command.lineWidth;
    if (typeof command.font === "string") ctx.font = command.font;
    if (typeof command.alpha === "number") ctx.globalAlpha = command.alpha;
    if (typeof command.composite === "string")
      ctx.globalCompositeOperation = command.composite;
    if (typeof command.lineCap === "string") ctx.lineCap = command.lineCap;
    if (typeof command.lineJoin === "string") ctx.lineJoin = command.lineJoin;
    if (typeof command.miterLimit === "number")
      ctx.miterLimit = command.miterLimit;
    if (typeof command.filter === "string") ctx.filter = command.filter;
    if (typeof command.direction === "string")
      ctx.direction = command.direction;
    if (typeof command.textAlign === "string")
      ctx.textAlign = command.textAlign;
    if (typeof command.textBaseline === "string")
      ctx.textBaseline = command.textBaseline;
  }

  private defineGradient(
    command: Extract<CanvasCommand, { type: "gradient" }>,
  ): void {
    const ctx = this.ctx;
    if (!ctx || !command.id) return;
    let gradient: CanvasGradient;
    if (command.kind === "radial") {
      gradient = ctx.createRadialGradient(
        num(command.x0),
        num(command.y0),
        num(command.r0),
        num(command.x1),
        num(command.y1),
        num(command.r1),
      );
    } else if (command.kind === "conic" && "createConicGradient" in ctx) {
      gradient = ctx.createConicGradient(
        num(command.startAngle),
        num(command.x),
        num(command.y),
      );
    } else {
      gradient = ctx.createLinearGradient(
        num(command.x0),
        num(command.y0),
        num(command.x1),
        num(command.y1),
      );
    }
    for (const stop of command.stops ?? [])
      gradient.addColorStop(clamp(stop.offset, 0, 1), stop.color);
    this.resources.set(command.id, gradient);
  }

  private definePattern(
    command: Extract<CanvasCommand, { type: "pattern" }>,
    version: number,
  ): void {
    const ctx = this.ctx;
    if (!ctx || !command.id || !command.src) return;
    const image = this.loadImage(command.src, () =>
      this.rerenderIfCurrent(version),
    );
    if (!image?.complete) return;
    const pattern = ctx.createPattern(image, command.repetition || "repeat");
    if (pattern) this.resources.set(command.id, pattern);
  }

  private clip(command: Extract<CanvasCommand, { type: "clip" }>): void {
    const ctx = this.ctx;
    const path = this.pathFromShape(command);
    if (!ctx || !path) return;
    ctx.clip(path, command.fillRule || "nonzero");
  }

  private drawRect(command: Extract<CanvasCommand, { type: "rect" }>): void {
    const ctx = this.ctx;
    if (!ctx) return;
    ctx.save();
    this.applyStyle(command);
    ctx.beginPath();
    addRectPath(
      ctx,
      num(command.x),
      num(command.y),
      num(command.width),
      num(command.height),
      num(command.radius),
    );
    this.fillStroke(command);
    ctx.restore();
  }

  private drawLine(command: Extract<CanvasCommand, { type: "line" }>): void {
    const ctx = this.ctx;
    if (!ctx) return;
    ctx.save();
    this.applyStyle(command);
    ctx.beginPath();
    ctx.moveTo(num(command.x1), num(command.y1));
    ctx.lineTo(num(command.x2), num(command.y2));
    ctx.stroke();
    ctx.restore();
  }

  private drawArc(command: Extract<CanvasCommand, { type: "arc" }>): void {
    const ctx = this.ctx;
    if (!ctx) return;
    ctx.save();
    this.applyStyle(command);
    ctx.beginPath();
    ctx.arc(
      num(command.x),
      num(command.y),
      num(command.radius),
      num(command.startAngle),
      num(command.endAngle, Math.PI * 2),
      command.counterclockwise,
    );
    this.fillStroke(command);
    ctx.restore();
  }

  private drawQuadratic(
    command: Extract<CanvasCommand, { type: "quadraticCurve" }>,
  ): void {
    const ctx = this.ctx;
    if (!ctx) return;
    ctx.save();
    this.applyStyle(command);
    ctx.beginPath();
    ctx.moveTo(num(command.x0), num(command.y0));
    ctx.quadraticCurveTo(
      num(command.cpx),
      num(command.cpy),
      num(command.x),
      num(command.y),
    );
    this.fillStroke(command, true);
    ctx.restore();
  }

  private drawBezier(
    command: Extract<CanvasCommand, { type: "bezierCurve" }>,
  ): void {
    const ctx = this.ctx;
    if (!ctx) return;
    ctx.save();
    this.applyStyle(command);
    ctx.beginPath();
    ctx.moveTo(num(command.x0), num(command.y0));
    ctx.bezierCurveTo(
      num(command.cp1x),
      num(command.cp1y),
      num(command.cp2x),
      num(command.cp2y),
      num(command.x),
      num(command.y),
    );
    this.fillStroke(command, true);
    ctx.restore();
  }

  private drawPolyline(
    command: Extract<CanvasCommand, { type: "polyline" | "polygon" }>,
    close: boolean,
  ): void {
    const ctx = this.ctx;
    const points = command.points ?? [];
    if (!ctx || points.length === 0) return;
    ctx.save();
    this.applyStyle(command);
    ctx.beginPath();
    ctx.moveTo(num(points[0].x), num(points[0].y));
    for (let index = 1; index < points.length; index++) {
      const point = points[index];
      ctx.lineTo(num(point.x), num(point.y));
    }
    if (close) ctx.closePath();
    this.fillStroke(command, !close);
    ctx.restore();
  }

  private drawCircle(
    command: Extract<CanvasCommand, { type: "circle" }>,
  ): void {
    const ctx = this.ctx;
    if (!ctx) return;
    ctx.save();
    this.applyStyle(command);
    ctx.beginPath();
    ctx.arc(
      num(command.x),
      num(command.y),
      num(command.radius),
      0,
      Math.PI * 2,
    );
    this.fillStroke(command);
    ctx.restore();
  }

  private drawEllipse(
    command: Extract<CanvasCommand, { type: "ellipse" }>,
  ): void {
    const ctx = this.ctx;
    if (!ctx) return;
    ctx.save();
    this.applyStyle(command);
    ctx.beginPath();
    ctx.ellipse(
      num(command.x),
      num(command.y),
      num(command.radiusX),
      num(command.radiusY),
      num(command.rotation),
      0,
      Math.PI * 2,
    );
    this.fillStroke(command);
    ctx.restore();
  }

  private drawPath(command: Extract<CanvasCommand, { type: "path" }>): void {
    const ctx = this.ctx;
    if (!ctx || !command.d) return;
    const path = this.pathFromData(command.d);
    if (!path) return;
    ctx.save();
    this.applyStyle(command);
    if (command.fill !== false) ctx.fill(path, command.fillRule || "nonzero");
    if (command.stroke !== false) ctx.stroke(path);
    ctx.restore();
  }

  private drawText(command: Extract<CanvasCommand, { type: "text" }>): void {
    const ctx = this.ctx;
    if (!ctx) return;
    ctx.save();
    ctx.textAlign = "start";
    ctx.textBaseline = "alphabetic";
    this.applyStyle(command);
    const text = str(command.text);
    const x = num(command.x);
    const y = num(command.y);
    const maxWidth =
      typeof command.maxWidth === "number" ? command.maxWidth : undefined;
    if (command.stroke) ctx.strokeText(text, x, y, maxWidth);
    if (command.fill !== false) ctx.fillText(text, x, y, maxWidth);
    ctx.restore();
  }

  private drawTextBox(
    command: Extract<CanvasCommand, { type: "textBox" }>,
  ): void {
    const ctx = this.ctx;
    if (!ctx) return;
    ctx.save();
    ctx.textAlign = "start";
    ctx.textBaseline = "alphabetic";
    this.applyStyle(command);
    const width = num(command.width, 240);
    const lines = this.wrapText(ctx, str(command.text), width);
    const lineHeight = num(command.lineHeight, 18);
    const x = textBoxAnchorX(num(command.x), width, ctx.textAlign);
    lines.forEach((line, index) => {
      const y = num(command.y) + index * lineHeight;
      if (command.stroke) ctx.strokeText(line, x, y);
      if (command.fill !== false) ctx.fillText(line, x, y);
    });
    ctx.restore();
  }

  private measureText(
    command: Extract<CanvasCommand, { type: "measureText" }>,
  ): void {
    const ctx = this.ctx;
    if (!ctx) return;
    if (command.font) ctx.font = command.font;
    const metrics = ctx.measureText(str(command.text));
    this.emit({
      type: "textMetrics",
      requestId: command.requestId,
      text: command.text,
      width: metrics.width,
      actualBoundingBoxLeft: metrics.actualBoundingBoxLeft,
      actualBoundingBoxRight: metrics.actualBoundingBoxRight,
      actualBoundingBoxAscent: metrics.actualBoundingBoxAscent,
      actualBoundingBoxDescent: metrics.actualBoundingBoxDescent,
      fontBoundingBoxAscent: metrics.fontBoundingBoxAscent,
      fontBoundingBoxDescent: metrics.fontBoundingBoxDescent,
      emHeightAscent: metrics.emHeightAscent,
      emHeightDescent: metrics.emHeightDescent,
      hangingBaseline: metrics.hangingBaseline,
      alphabeticBaseline: metrics.alphabeticBaseline,
      ideographicBaseline: metrics.ideographicBaseline,
    });
  }

  private drawImage(
    command: Extract<CanvasCommand, { type: "image" }>,
    version: number,
  ): void {
    const ctx = this.ctx;
    if (!ctx || !command.src) return;
    const image = this.loadImage(command.src, () =>
      this.rerenderIfCurrent(version),
    );
    if (!image?.complete) return;
    ctx.save();
    this.applyStyle(command);
    const previousSmoothing = ctx.imageSmoothingEnabled;
    if (typeof command.smoothing === "boolean")
      ctx.imageSmoothingEnabled = command.smoothing;
    const sw = num(command.sourceWidth);
    const sh = num(command.sourceHeight);
    if (sw > 0 && sh > 0) {
      ctx.drawImage(
        image,
        num(command.sourceX),
        num(command.sourceY),
        sw,
        sh,
        num(command.x),
        num(command.y),
        num(command.width, sw),
        num(command.height, sh),
      );
    } else {
      ctx.drawImage(
        image,
        num(command.x),
        num(command.y),
        num(command.width, image.naturalWidth || image.width),
        num(command.height, image.naturalHeight || image.height),
      );
    }
    ctx.imageSmoothingEnabled = previousSmoothing;
    ctx.restore();
  }

  private drawImageData(
    command: Extract<CanvasCommand, { type: "imageData" }>,
  ): void {
    const ctx = this.ctx;
    const width = Math.max(0, Math.floor(num(command.width)));
    const height = Math.max(0, Math.floor(num(command.height)));
    if (!ctx || width === 0 || height === 0 || !Array.isArray(command.data))
      return;
    const data = new Uint8ClampedArray(width * height * 4);
    for (let i = 0; i < data.length && i < command.data.length; i++)
      data[i] = clamp(command.data[i], 0, 255);
    ctx.putImageData(
      new ImageData(data, width, height),
      num(command.x),
      num(command.y),
    );
  }

  private snapshot(
    command: Extract<CanvasCommand, { type: "snapshot" }>,
  ): void {
    if (!this.canvas) return;
    const key = command.requestId || "__default__";
    const minInterval = Math.max(0, command.minIntervalMs ?? 500);
    const now = performance.now();
    const last = this.snapshotTimes.get(key);
    if (last !== undefined && now - last < minInterval) return;
    this.snapshotTimes.set(key, now);
    const mime = command.mime || "image/png";
    this.emit({
      type: "snapshot",
      requestId: command.requestId,
      mime,
      dataUrl: this.canvas.toDataURL(mime, command.quality),
      width: this.width,
      height: this.height,
    });
  }

  private fillStroke(command: CanvasPaint, strokeDefault = false): void {
    const ctx = this.ctx;
    if (!ctx) return;
    if (command.fill !== false) ctx.fill();
    if (
      command.stroke !== false &&
      (strokeDefault || command.stroke || command.strokeId)
    )
      ctx.stroke();
  }

  private pathFromShape(shape: {
    shape?: string;
    d?: string;
    x?: number;
    y?: number;
    width?: number;
    height?: number;
    radius?: number;
    points?: CanvasPoint[];
  }): Path2D | undefined {
    const path = new Path2D();
    if (shape.d) return this.pathFromData(shape.d);
    if (shape.shape === "circle") {
      path.arc(num(shape.x), num(shape.y), num(shape.radius), 0, Math.PI * 2);
      return path;
    }
    if (shape.shape === "polygon" && shape.points?.length) {
      path.moveTo(num(shape.points[0].x), num(shape.points[0].y));
      for (let index = 1; index < shape.points.length; index++) {
        const point = shape.points[index];
        path.lineTo(num(point.x), num(point.y));
      }
      path.closePath();
      return path;
    }
    addRectPath(
      path,
      num(shape.x),
      num(shape.y),
      num(shape.width),
      num(shape.height),
      num(shape.radius),
    );
    return path;
  }

  private regionContains(region: CanvasRegion, point: CanvasPoint): boolean {
    if (region.shape === "circle") {
      return (
        Math.hypot(point.x - region.x, point.y - region.y) <= num(region.radius)
      );
    }
    if (region.shape === "polygon" && region.points?.length)
      return pointInPolygon(point, region.points);
    if ((region.shape === "path" || region.d) && this.ctx && region.d)
      return this.isPointInPath(region.d, point);
    return (
      point.x >= region.x &&
      point.x <= region.x + num(region.width) &&
      point.y >= region.y &&
      point.y <= region.y + num(region.height)
    );
  }

  private loadImage(
    src: string,
    onload: () => void,
  ): HTMLImageElement | undefined {
    let image = this.imageCache.get(src);
    if (!image) {
      image = new Image();
      image.crossOrigin = "anonymous";
      image.onload = onload;
      image.onerror = () => this.imageCache.delete(src);
      image.src = src;
      this.imageCache.set(src, image);
    }
    return image;
  }

  private rerenderIfCurrent(version: number): void {
    if (version !== this.frameVersion || !this.lastFrame) return;
    this.paint(this.lastFrame, this.lastBackground, version);
  }

  private isPointInPath(d: string, point: CanvasPoint): boolean {
    if (!this.ctx) return false;
    const path = this.pathFromData(d);
    return path ? this.ctx.isPointInPath(path, point.x, point.y) : false;
  }

  private pathFromData(d: string): Path2D | undefined {
    if (this.pathCache.has(d)) return this.pathCache.get(d) ?? undefined;
    try {
      const path = new Path2D(d);
      remember(this.pathCache, d, path, MAX_PATH_CACHE);
      return path;
    } catch {
      remember(this.pathCache, d, null, MAX_PATH_CACHE);
      return undefined;
    }
  }

  private wrapText(
    ctx: CanvasRenderingContext2D,
    text: string,
    width: number,
  ): string[] {
    const key = `${ctx.font}\u0000${width}\u0000${text}`;
    const cached = this.textWrapCache.get(key);
    if (cached) return cached;
    const lines = wrapText(ctx, text, width);
    remember(this.textWrapCache, key, lines, MAX_TEXT_WRAP_CACHE);
    return lines;
  }
}

function addRectPath(
  path: Pick<CanvasRenderingContext2D, "rect"> | Path2D,
  x: number,
  y: number,
  width: number,
  height: number,
  radius = 0,
): void {
  if (radius > 0 && "roundRect" in path)
    path.roundRect(x, y, width, height, radius);
  else path.rect(x, y, width, height);
}

function wrapText(
  ctx: CanvasRenderingContext2D,
  text: string,
  width: number,
): string[] {
  const words = text.split(/\s+/).filter(Boolean);
  const lines: string[] = [];
  let current = "";
  for (const word of words) {
    const next = current ? `${current} ${word}` : word;
    if (current && ctx.measureText(next).width > width) {
      lines.push(current);
      current = word;
    } else {
      current = next;
    }
  }
  if (current) lines.push(current);
  return lines.length ? lines : [""];
}

function textBoxAnchorX(
  x: number,
  width: number,
  align: CanvasTextAlign,
): number {
  if (align === "center") return x + width / 2;
  if (align === "right" || align === "end") return x + width;
  return x;
}

function pointInPolygon(point: CanvasPoint, polygon: CanvasPoint[]): boolean {
  let inside = false;
  for (let i = 0, j = polygon.length - 1; i < polygon.length; j = i++) {
    const xi = polygon[i].x;
    const yi = polygon[i].y;
    const xj = polygon[j].x;
    const yj = polygon[j].y;
    if (
      yi > point.y !== yj > point.y &&
      point.x < ((xj - xi) * (point.y - yi)) / (yj - yi) + xi
    )
      inside = !inside;
  }
  return inside;
}

function isCanvasStyle(
  value: unknown,
): value is CanvasGradient | CanvasPattern {
  return typeof value === "object" && value !== null;
}

function remember<K, V>(cache: Map<K, V>, key: K, value: V, max: number): void {
  if (cache.size >= max) {
    const oldest = cache.keys().next();
    if (!oldest.done) cache.delete(oldest.value);
  }
  cache.set(key, value);
}

function num(value: unknown, fallback = 0): number {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}

function str(value: unknown): string {
  return typeof value === "string" ? value : String(value ?? "");
}

function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, value));
}
