import type { ButtonPassThroughMethodOptions } from "primevue/button";
import { cn } from "../utils/cn";

export const fieldSurface =
  "rounded-md border border-surface-300 bg-surface-0 dark:border-surface-700 dark:bg-surface-950";
const focusRing =
  "focus:border-primary-500 focus:ring-2 focus:ring-primary-500/30";
const focusWithinRing =
  "focus-within:border-primary-500 focus-within:ring-2 focus-within:ring-primary-500/30";

const inputBase = cn(
  "w-full px-2.5 py-1.5 text-sm text-surface-800 outline-none transition duration-150 placeholder:text-surface-400 dark:text-surface-100",
  fieldSurface,
  focusRing,
);

export const inputClass = inputBase;

export const searchInputClass = cn(
  "w-full py-1.5 pl-9 pr-3 text-sm text-surface-800 outline-none transition duration-150 placeholder:text-surface-400 dark:text-surface-100",
  fieldSurface,
  focusRing,
);

export const dialogRoot = (maxWidth = "max-w-md"): string =>
  cn(
    "flex max-h-[calc(100vh-2rem)] w-full flex-col overflow-hidden rounded-xl border border-surface-200 bg-surface-0 shadow-2xl ring-1 ring-surface-950/5 dark:border-surface-800 dark:bg-surface-900 dark:ring-surface-0/5",
    maxWidth,
  );

export const drawerRoot = (maxWidth = "max-w-md"): string =>
  cn(
    "fixed right-0 top-0 z-50 flex h-dvh w-full flex-col overflow-hidden border-l border-surface-200 bg-surface-0 text-surface-900 shadow-2xl ring-1 ring-surface-950/5 dark:border-surface-800 dark:bg-surface-950 dark:text-surface-0 dark:ring-surface-0/5",
    maxWidth,
  );

export const btnPrimary =
  "inline-flex min-w-0 items-center justify-center gap-1.5 rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-primary-700 disabled:opacity-50";
export const btnGhost =
  "inline-flex min-w-0 items-center justify-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium text-surface-600 transition-colors hover:bg-surface-100 disabled:opacity-50 dark:text-surface-300 dark:hover:bg-surface-800";
export const btnDanger =
  "inline-flex min-w-0 items-center justify-center gap-1.5 rounded-md bg-rose-600 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-rose-700 disabled:opacity-50";
export const btnPrimaryBlock =
  "flex w-full items-center justify-center gap-1.5 rounded-md bg-primary-600 px-4 py-2.5 text-sm font-medium text-white shadow-sm transition-colors hover:bg-primary-700 focus-visible:ring-2 focus-visible:ring-primary-500/40 disabled:opacity-50";

const buttonBase =
  "inline-flex min-w-0 items-center justify-center gap-1.5 whitespace-nowrap rounded-md text-sm font-medium outline-none transition-colors focus-visible:ring-2 focus-visible:ring-primary-500/40 disabled:pointer-events-none disabled:opacity-50";

const stepperButton =
  "inline-flex w-9 shrink-0 cursor-pointer items-center justify-center rounded-md border border-surface-300 text-surface-600 outline-none transition-colors hover:bg-surface-100 focus-visible:ring-2 focus-visible:ring-primary-500/40 disabled:pointer-events-none disabled:opacity-40 dark:border-surface-700 dark:text-surface-300 dark:hover:bg-surface-800";
const buttonSize = {
  small: "px-2.5 py-1 text-xs",
  normal: "px-3 py-1.5",
  large: "px-4 py-2 text-base",
};
const buttonSolid = {
  primary: "bg-primary-600 text-white hover:bg-primary-700",
  secondary:
    "border border-surface-300 bg-surface-0 text-surface-700 hover:bg-surface-100 dark:border-surface-700 dark:bg-surface-950 dark:text-surface-200 dark:hover:bg-surface-800",
  success: "bg-emerald-600 text-white hover:bg-emerald-700",
  info: "bg-sky-600 text-white hover:bg-sky-700",
  warn: "bg-amber-600 text-white hover:bg-amber-700",
  help: "bg-violet-600 text-white hover:bg-violet-700",
  danger: "bg-rose-600 text-white hover:bg-rose-700",
  contrast:
    "bg-surface-900 text-white hover:bg-surface-800 dark:bg-surface-100 dark:text-surface-950 dark:hover:bg-surface-200",
};
const buttonText = {
  primary:
    "text-primary-700 hover:bg-primary-50 dark:text-primary-300 dark:hover:bg-primary-500/10",
  secondary:
    "text-surface-600 hover:bg-surface-100 dark:text-surface-300 dark:hover:bg-surface-800",
  success:
    "text-emerald-700 hover:bg-emerald-50 dark:text-emerald-300 dark:hover:bg-emerald-500/10",
  info: "text-sky-700 hover:bg-sky-50 dark:text-sky-300 dark:hover:bg-sky-500/10",
  warn: "text-amber-700 hover:bg-amber-50 dark:text-amber-300 dark:hover:bg-amber-500/10",
  help: "text-violet-700 hover:bg-violet-50 dark:text-violet-300 dark:hover:bg-violet-500/10",
  danger:
    "text-rose-700 hover:bg-rose-50 dark:text-rose-300 dark:hover:bg-rose-500/10",
  contrast:
    "text-surface-900 hover:bg-surface-100 dark:text-surface-100 dark:hover:bg-surface-800",
};
const buttonOutlined = {
  primary:
    "border border-primary-300 text-primary-700 hover:bg-primary-50 dark:border-primary-700 dark:text-primary-300 dark:hover:bg-primary-500/10",
  secondary:
    "border border-surface-300 text-surface-700 hover:bg-surface-100 dark:border-surface-700 dark:text-surface-200 dark:hover:bg-surface-800",
  success:
    "border border-emerald-300 text-emerald-700 hover:bg-emerald-50 dark:border-emerald-700 dark:text-emerald-300 dark:hover:bg-emerald-500/10",
  info: "border border-sky-300 text-sky-700 hover:bg-sky-50 dark:border-sky-700 dark:text-sky-300 dark:hover:bg-sky-500/10",
  warn: "border border-amber-300 text-amber-700 hover:bg-amber-50 dark:border-amber-700 dark:text-amber-300 dark:hover:bg-amber-500/10",
  help: "border border-violet-300 text-violet-700 hover:bg-violet-50 dark:border-violet-700 dark:text-violet-300 dark:hover:bg-violet-500/10",
  danger:
    "border border-rose-300 text-rose-700 hover:bg-rose-50 dark:border-rose-700 dark:text-rose-300 dark:hover:bg-rose-500/10",
  contrast:
    "border border-surface-600 text-surface-900 hover:bg-surface-100 dark:border-surface-400 dark:text-surface-100 dark:hover:bg-surface-800",
};

type ButtonTone = keyof typeof buttonSolid;
type ButtonProps = ButtonPassThroughMethodOptions<unknown>["props"];

const buttonTones = [
  "secondary",
  "success",
  "info",
  "warn",
  "help",
  "danger",
  "contrast",
] as const;

function buttonTone(severity: ButtonProps["severity"]): ButtonTone {
  return (buttonTones as readonly string[]).includes(severity ?? "")
    ? (severity as ButtonTone)
    : "primary";
}

function buttonSizeClass(size: ButtonProps["size"]): string {
  switch (size) {
    case "small":
      return buttonSize.small;
    case "large":
      return buttonSize.large;
    default:
      return buttonSize.normal;
  }
}

function buttonShapeClass(rounded: ButtonProps["rounded"]): string {
  return rounded ? "rounded-full" : "rounded-md";
}

function buttonRoot(options: ButtonPassThroughMethodOptions<unknown>): string {
  const props = options.props;
  const tone = buttonTone(props.severity);
  const size = buttonSizeClass(props.size);
  const shape = buttonShapeClass(props.rounded);
  const width = props.fluid && "w-full";
  const variant = props.variant;
  if (props.link || variant === "link") {
    return cn(
      buttonBase,
      shape,
      width,
      "px-0 py-0 text-primary-700 hover:text-primary-800 hover:underline dark:text-primary-300 dark:hover:text-primary-200",
    );
  }
  if (props.text || variant === "text") {
    return cn(buttonBase, shape, width, size, buttonText[tone]);
  }
  if (props.outlined || variant === "outlined") {
    return cn(buttonBase, shape, width, size, buttonOutlined[tone]);
  }
  return cn(buttonBase, shape, width, size, buttonSolid[tone]);
}

const overlay =
  "mt-1.5 origin-top overflow-hidden rounded-lg border border-surface-200 bg-surface-0 p-1 shadow-lg ring-1 ring-surface-950/5 dark:border-surface-700 dark:bg-surface-900 dark:ring-surface-0/5";
const option =
  "cursor-pointer truncate rounded-md px-2.5 py-1.5 text-sm text-surface-700 transition-colors data-[p-focused=true]:bg-surface-100 data-[p-selected=true]:bg-primary-50 data-[p-selected=true]:font-medium data-[p-selected=true]:text-primary-700 dark:text-surface-200 dark:data-[p-focused=true]:bg-surface-800 dark:data-[p-selected=true]:bg-primary-500/15 dark:data-[p-selected=true]:text-primary-300";
const overlayTransition = {
  enterFromClass: "opacity-0 scale-95",
  enterActiveClass: "transition duration-100 ease-out",
  enterToClass: "opacity-100 scale-100",
  leaveActiveClass: "transition duration-75 ease-in",
  leaveToClass: "opacity-0",
};

const dialogMask =
  "pointer-events-auto fixed inset-0 flex items-center justify-center bg-surface-950/50 p-4 backdrop-blur-sm";
const dialogHeader =
  "flex shrink-0 items-center justify-between border-b border-surface-200 px-5 py-3.5 dark:border-surface-800";
const dialogTitle =
  "text-base font-semibold tracking-tight text-surface-900 dark:text-surface-0";
const dialogFooter =
  "flex shrink-0 items-center justify-end gap-2 border-t border-surface-200 px-5 py-3.5 dark:border-surface-800";
const dialogTransition = {
  enterFromClass: "opacity-0 translate-y-2 scale-[0.97]",
  enterActiveClass: "transition duration-200 ease-[cubic-bezier(0.16,1,0.3,1)]",
  enterToClass: "opacity-100 translate-y-0 scale-100",
  leaveFromClass: "opacity-100 translate-y-0 scale-100",
  leaveActiveClass: "transition duration-150 ease-in",
  leaveToClass: "opacity-0 translate-y-1 scale-[0.98]",
};

const checkbox = {
  root: "relative inline-flex h-4 w-4 shrink-0",
  input: "absolute inset-0 cursor-pointer opacity-0",
  box: "flex h-4 w-4 items-center justify-center rounded border border-surface-300 bg-surface-0 transition-colors dark:border-surface-600 dark:bg-surface-950 data-[p~=checked]:border-primary-500 data-[p~=checked]:bg-primary-500 data-[p~=checked]:text-white dark:data-[p~=checked]:border-primary-500 dark:data-[p~=checked]:bg-primary-500",
  icon: "h-3 w-3 text-white",
};

const radioButton = {
  root: "relative inline-flex h-4 w-4 shrink-0",
  input: "absolute inset-0 cursor-pointer opacity-0",
  box: "flex h-4 w-4 items-center justify-center rounded-full border border-surface-300 bg-surface-0 transition-colors dark:border-surface-600 dark:bg-surface-950 data-[p~=checked]:border-primary-500 data-[p~=checked]:bg-primary-500 dark:data-[p~=checked]:border-primary-500 dark:data-[p~=checked]:bg-primary-500",
  icon: "h-2 w-2 rounded-full bg-white",
};

type SeverityOptions = { props?: { severity?: string } };

const severitySurface: Record<string, string> = {
  success:
    "border-emerald-500/30 bg-emerald-50 text-emerald-800 dark:bg-emerald-950/60 dark:text-emerald-200",
  info: "border-sky-500/30 bg-sky-50 text-sky-800 dark:bg-sky-950/60 dark:text-sky-200",
  warn: "border-amber-500/30 bg-amber-50 text-amber-800 dark:bg-amber-950/60 dark:text-amber-200",
  error:
    "border-rose-500/30 bg-rose-50 text-rose-800 dark:bg-rose-950/60 dark:text-rose-200",
  danger:
    "border-rose-500/30 bg-rose-50 text-rose-800 dark:bg-rose-950/60 dark:text-rose-200",
  secondary:
    "border-surface-300 bg-surface-100 text-surface-700 dark:border-surface-700 dark:bg-surface-800 dark:text-surface-200",
  contrast:
    "border-surface-900 bg-surface-900 text-white dark:border-surface-100 dark:bg-surface-100 dark:text-surface-950",
};

function severitySurfaceClass(options: SeverityOptions): string {
  return severitySurface[options.props?.severity ?? ""] ?? severitySurface.info;
}

function tagRoot(options: SeverityOptions): string {
  return cn(
    "inline-flex items-center gap-1 rounded px-2 py-0.5 text-xs font-medium",
    severitySurfaceClass(options),
  );
}

function messageRoot(options: SeverityOptions): string {
  return cn(
    "flex items-start gap-2 rounded-md border px-3 py-2 text-sm",
    severitySurfaceClass(options),
  );
}

const paginatorButton =
  "inline-flex h-8 min-w-8 cursor-pointer items-center justify-center rounded-md px-2 text-sm text-surface-600 transition-colors hover:bg-surface-100 disabled:pointer-events-none disabled:opacity-40 data-[p-active=true]:bg-primary-600 data-[p-active=true]:font-medium data-[p-active=true]:text-white data-[p-active=true]:hover:bg-primary-700 dark:text-surface-300 dark:hover:bg-surface-800 dark:data-[p-active=true]:bg-primary-500 dark:data-[p-active=true]:text-white dark:data-[p-active=true]:hover:bg-primary-400";
const paginator = {
  root: "flex flex-wrap items-center justify-end gap-2 border-t border-surface-200 bg-surface-0 px-3 py-2 dark:border-surface-800 dark:bg-surface-950",
  content: "flex flex-wrap items-center gap-1",
  pages: "flex items-center gap-1",
  first: paginatorButton,
  prev: paginatorButton,
  next: paginatorButton,
  last: paginatorButton,
  page: paginatorButton,
  firstIcon: "h-4 w-4",
  prevIcon: "h-4 w-4",
  nextIcon: "h-4 w-4",
  lastIcon: "h-4 w-4",
  current: "px-2 text-xs text-surface-500 dark:text-surface-400",
  pcRowPerPageDropdown: {
    root: cn(
      "flex min-w-20 items-center justify-between text-sm transition duration-150",
      fieldSurface,
      focusWithinRing,
    ),
    label:
      "min-w-0 flex-1 truncate px-2.5 py-1.5 text-left text-surface-800 dark:text-surface-100",
    dropdown: "shrink-0 px-2 text-surface-400",
    overlay,
    transition: overlayTransition,
    listContainer: "max-h-60 overflow-auto p-1",
    option,
  },
};

export const primeVuePassthrough = {
  inputtext: { root: inputBase },
  breadcrumb: {
    root: "w-full",
    list: "flex flex-wrap items-center gap-1 text-sm",
    item: "inline-flex items-center",
    separator:
      "mx-0.5 inline-flex items-center text-surface-300 dark:text-surface-600",
  },
  chart: { root: "relative h-full w-full" },
  textarea: { root: cn(inputBase, "min-h-20 font-mono") },
  inputnumber: {
    root: "flex w-full items-stretch gap-1.5",
    pcInputText: { root: cn(inputBase, "min-w-0 flex-1") },
    incrementButton: stepperButton,
    decrementButton: stepperButton,
  },
  password: {
    root: "relative block w-full",
    pcInputText: { root: cn(inputBase, "pr-9") },
    maskIcon:
      "absolute right-3 top-1/2 -translate-y-1/2 cursor-pointer text-surface-400 transition-colors hover:text-surface-600 dark:hover:text-surface-300",
    unmaskIcon:
      "absolute right-3 top-1/2 -translate-y-1/2 cursor-pointer text-surface-400 transition-colors hover:text-surface-600 dark:hover:text-surface-300",
  },

  select: {
    root: cn(
      "flex w-full min-w-0 items-center justify-between text-sm transition duration-150",
      fieldSurface,
      focusWithinRing,
    ),
    label:
      "min-w-0 flex-1 truncate px-2.5 py-1.5 text-left text-surface-800 dark:text-surface-100",
    dropdown: "shrink-0 px-2 text-surface-400",
    overlay,
    transition: overlayTransition,
    listContainer: "max-h-60 overflow-auto p-1",
    option,
    emptyMessage: "px-3 py-2 text-sm text-surface-400",
  },

  checkbox,
  radiobutton: radioButton,
  multiselect: {
    root: cn(
      "flex w-full items-center justify-between text-sm transition duration-150",
      fieldSurface,
      focusWithinRing,
    ),
    labelContainer: "min-w-0 flex-1 overflow-hidden",
    label:
      "flex min-h-9 flex-wrap items-center gap-1.5 px-2 py-1.5 text-left text-surface-500 dark:text-surface-400",
    dropdown: "shrink-0 px-2 text-surface-400",
    overlay,
    transition: overlayTransition,
    header:
      "flex items-center gap-2 border-b border-surface-200 px-3 py-2 dark:border-surface-800",
    pcHeaderCheckbox: checkbox,
    listContainer: "max-h-60 overflow-auto p-1",
    option:
      "flex cursor-pointer items-center gap-2.5 rounded-md px-2.5 py-1.5 text-sm text-surface-700 transition-colors data-[p-focused=true]:bg-surface-100 dark:text-surface-200 dark:data-[p-focused=true]:bg-surface-800",
    optionLabel: "min-w-0 flex-1 truncate",
    pcOptionCheckbox: checkbox,
    emptyMessage: "px-3 py-2 text-sm text-surface-400",
    pcChip: {
      root: "inline-flex items-center gap-1 rounded bg-surface-100 py-0.5 pl-2 pr-1 text-xs text-surface-700 dark:bg-surface-800 dark:text-surface-200",
      removeIcon:
        "h-3.5 w-3.5 cursor-pointer text-surface-400 transition-colors hover:text-surface-700 dark:hover:text-surface-200",
    },
  },

  autocomplete: {
    root: "relative block w-full",
    pcInputText: { root: inputBase },
    dropdown:
      "absolute right-0 top-0 flex h-full items-center px-2 text-surface-400",
    overlay,
    transition: overlayTransition,
    listContainer: "max-h-60 overflow-auto p-1",
    option,
    optionLabel: "min-w-0 flex-1 truncate",
    emptyMessage: "px-3 py-2 text-sm text-surface-400",
  },

  fileupload: {
    root: "inline-flex items-center",
    input: "sr-only",
    basicContent: "inline-flex items-center gap-2",
    pcChooseButton: {
      root: cn(
        buttonBase,
        buttonSize.normal,
        buttonSolid.secondary,
        "cursor-pointer",
      ),
    },
  },

  progressbar: {
    root: "relative overflow-hidden rounded-full bg-surface-200 dark:bg-surface-800",
    value: "h-full rounded-full bg-primary-500 transition-[width] duration-150",
    label: "hidden",
  },

  slider: {
    root: "relative h-5 w-full before:absolute before:left-0 before:top-1/2 before:h-1 before:w-full before:-translate-y-1/2 before:rounded-full before:bg-surface-200 dark:before:bg-surface-700",
    range: "absolute top-1/2 h-1 -translate-y-1/2 rounded-full bg-primary-500",
    handle:
      "absolute top-1/2 h-4 w-4 -translate-y-1/2 rounded-full border-2 border-primary-500 bg-surface-0 shadow-sm outline-none transition-shadow focus-visible:ring-2 focus-visible:ring-primary-500/40 dark:bg-surface-950",
  },

  toggleswitch: {
    root: "relative inline-flex h-5 w-9 cursor-pointer",
    input: "absolute inset-0 z-10 cursor-pointer opacity-0",
    slider:
      "absolute inset-0 rounded-full bg-surface-300 transition-colors before:absolute before:left-0.5 before:top-0.5 before:h-4 before:w-4 before:rounded-full before:bg-white before:transition-transform data-[p~=checked]:bg-primary-500 data-[p~=checked]:before:translate-x-4 dark:bg-surface-700 dark:data-[p~=checked]:bg-primary-500",
  },

  button: {
    root: buttonRoot,
    icon: "h-4 w-4 shrink-0",
    loadingIcon: "h-4 w-4 shrink-0 animate-spin",
    label: "min-w-0 truncate",
  },

  badge: {
    root: tagRoot,
  },

  tag: {
    root: tagRoot,
    icon: "h-3.5 w-3.5",
    label: "truncate",
  },

  message: {
    root: messageRoot,
    contentWrapper: "flex min-w-0 flex-1 items-start gap-2",
    content: "flex min-w-0 flex-1 items-start gap-2",
    icon: "mt-0.5 h-4 w-4 shrink-0",
    text: "min-w-0 flex-1",
    closeButton:
      "ml-auto shrink-0 rounded p-0.5 text-current/60 transition-colors hover:bg-black/5 hover:text-current dark:hover:bg-white/10",
    closeIcon: "h-4 w-4",
  },

  toast: {
    root: "fixed z-[100] flex w-80 max-w-[calc(100vw-2rem)] flex-col gap-2",
    message: "",
  },

  menu: {
    root: overlay,
    list: "flex min-w-40 flex-col gap-0.5",
    itemContent:
      "flex cursor-pointer items-center gap-2 rounded-md px-2.5 py-1.5 text-sm text-surface-700 transition-colors hover:bg-surface-100 dark:text-surface-200 dark:hover:bg-surface-800",
    itemLabel: "min-w-0 flex-1 truncate",
    separator: "my-1 border-t border-surface-200 dark:border-surface-800",
  },

  selectbutton: {
    root: "inline-flex gap-0.5 rounded-md border border-surface-300 bg-surface-0 p-0.5 dark:border-surface-700 dark:bg-surface-950",
    pcToggleButton: {
      root: "inline-flex h-8 w-8 cursor-pointer items-center justify-center rounded text-surface-500 transition-colors hover:bg-surface-100 hover:text-surface-800 data-[p-checked=true]:bg-surface-100 data-[p-checked=true]:text-surface-900 dark:hover:bg-surface-800 dark:hover:text-surface-100 dark:data-[p-checked=true]:bg-surface-800 dark:data-[p-checked=true]:text-surface-0",
      content: "inline-flex items-center justify-center",
      label: "sr-only",
    },
  },

  dialog: {
    mask: cn(dialogMask, "z-50"),
    root: dialogRoot(),
    header: dialogHeader,
    title: dialogTitle,
    content: "min-h-0 overflow-auto p-5",
    footer: dialogFooter,
    pcCloseButton: {
      root: "rounded-md p-1 text-surface-400 transition-colors hover:bg-surface-100 hover:text-surface-600 dark:hover:bg-surface-800 dark:hover:text-surface-200",
    },
    transition: dialogTransition,
  },

  drawer: {
    mask: "pointer-events-auto fixed inset-0 z-50 bg-surface-950/30 backdrop-blur-[1px]",
    root: drawerRoot(),
    header:
      "flex shrink-0 items-center justify-between border-b border-surface-200 px-4 py-3 dark:border-surface-800",
    title: "min-w-0 flex-1 truncate text-sm font-semibold",
    content: "min-h-0 flex-1 overflow-auto",
    pcCloseButton: {
      root: cn(buttonBase, "h-8 w-8 rounded-full", buttonText.secondary),
      icon: "h-4 w-4",
    },
    transition: {
      enterFromClass: "translate-x-full",
      enterActiveClass:
        "transition-transform duration-200 ease-[cubic-bezier(0.16,1,0.3,1)]",
      enterToClass: "translate-x-0",
      leaveFromClass: "translate-x-0",
      leaveActiveClass: "transition-transform duration-150 ease-in",
      leaveToClass: "translate-x-full",
    },
  },

  confirmdialog: {
    mask: cn(dialogMask, "z-[60]"),
    root: dialogRoot("max-w-md"),
    header: dialogHeader,
    title: dialogTitle,
    content: "min-h-0 overflow-auto px-5 py-5",
    footer: dialogFooter,
    transition: dialogTransition,
  },

  popover: {
    root: "z-50 mt-1.5 rounded-lg border border-surface-200 bg-surface-0 shadow-lg ring-1 ring-surface-950/5 dark:border-surface-700 dark:bg-surface-900 dark:ring-surface-0/5",
    content: "p-3",
    transition: overlayTransition,
  },

  card: {
    root: "overflow-hidden rounded-lg border border-surface-200 bg-surface-0 text-surface-800 shadow-sm ring-1 ring-surface-950/[0.02] dark:border-surface-800 dark:bg-surface-950 dark:text-surface-100 dark:ring-surface-0/[0.03]",
    header: "border-b border-surface-200 dark:border-surface-800",
    body: "p-4",
    caption: "mb-3",
    title: "text-base font-semibold text-surface-900 dark:text-surface-0",
    subtitle: "mt-1 text-sm text-surface-500 dark:text-surface-400",
    content: "text-sm",
    footer: "mt-4 flex items-center justify-end gap-2",
  },

  panel: {
    root: "rounded-lg border border-surface-200 bg-surface-0 text-surface-800 dark:border-surface-800 dark:bg-surface-950 dark:text-surface-100",
    header:
      "flex items-center justify-between gap-3 border-b border-surface-200 px-4 py-3 dark:border-surface-800",
    title: "min-w-0 flex-1 truncate text-sm font-semibold",
    headerActions: "flex shrink-0 items-center gap-1",
    pcToggleButton: {
      root: cn(buttonBase, "h-8 w-8 rounded-md", buttonText.secondary),
      icon: "h-4 w-4",
    },
    contentContainer: "overflow-hidden",
    contentWrapper: "p-4",
    content: "text-sm",
    footer: "border-t border-surface-200 px-4 py-3 dark:border-surface-800",
    transition: overlayTransition,
  },

  toolbar: {
    root: "flex flex-wrap items-center justify-between gap-3 rounded-md border border-surface-200 bg-surface-0 px-3 py-2 dark:border-surface-800 dark:bg-surface-950",
    start: "flex min-w-0 items-center gap-2",
    center: "flex min-w-0 items-center gap-2",
    end: "flex min-w-0 items-center justify-end gap-2",
  },

  divider: {
    root: "my-3 flex items-center border-0 text-xs font-medium text-surface-400 before:h-px before:flex-1 before:bg-surface-200 after:h-px after:flex-1 after:bg-surface-200 dark:before:bg-surface-800 dark:after:bg-surface-800",
    content:
      "mx-3 inline-flex shrink-0 items-center gap-1 bg-surface-0 px-1 dark:bg-surface-950",
  },

  skeleton: {
    root: "animate-pulse rounded-md bg-surface-100 dark:bg-surface-800",
  },

  progressspinner: {
    root: "h-8 w-8 text-primary-500",
    spin: "h-full w-full motion-safe:animate-spin [animation-duration:.8s]",
    circle: "fill-transparent stroke-current [stroke-width:3]",
  },

  datepicker: {
    root: "relative block w-full",
    pcInputText: { root: inputBase },
    dropdown:
      "absolute right-0 top-0 flex h-full items-center px-2 text-surface-400 transition-colors hover:text-surface-700 dark:hover:text-surface-200",
    dropdownIcon: "h-4 w-4",
    inputIconContainer:
      "absolute right-3 top-1/2 -translate-y-1/2 text-surface-400",
    inputIcon: "h-4 w-4",
    clearIcon:
      "absolute right-3 top-1/2 h-4 w-4 -translate-y-1/2 cursor-pointer text-surface-400 hover:text-surface-700 dark:hover:text-surface-200",
    panel: overlay,
    transition: overlayTransition,
    calendarContainer: "p-2",
    calendar: "min-w-64",
    header:
      "flex items-center justify-between gap-2 border-b border-surface-200 px-2 py-2 dark:border-surface-800",
    title: "flex items-center gap-1 text-sm font-medium",
    pcPrevButton: {
      root: cn(buttonBase, "h-8 w-8 rounded-md", buttonText.secondary),
      icon: "h-4 w-4",
    },
    pcNextButton: {
      root: cn(buttonBase, "h-8 w-8 rounded-md", buttonText.secondary),
      icon: "h-4 w-4",
    },
    selectMonth:
      "rounded px-2 py-1 text-sm transition-colors hover:bg-surface-100 dark:hover:bg-surface-800",
    selectYear:
      "rounded px-2 py-1 text-sm transition-colors hover:bg-surface-100 dark:hover:bg-surface-800",
    dayView: "w-full border-collapse",
    weekDay: "text-xs font-medium text-surface-400",
    dayCell: "p-0.5 text-center",
    day: "mx-auto flex h-8 w-8 items-center justify-center rounded-md text-sm transition-colors hover:bg-surface-100 data-[p-selected=true]:bg-primary-500 data-[p-selected=true]:text-white data-[p-today=true]:font-semibold dark:hover:bg-surface-800",
    monthView: "grid grid-cols-3 gap-1 p-2",
    month:
      "rounded-md px-3 py-2 text-center text-sm transition-colors hover:bg-surface-100 data-[p-selected=true]:bg-primary-500 data-[p-selected=true]:text-white dark:hover:bg-surface-800",
    yearView: "grid grid-cols-3 gap-1 p-2",
    year: "rounded-md px-3 py-2 text-center text-sm transition-colors hover:bg-surface-100 data-[p-selected=true]:bg-primary-500 data-[p-selected=true]:text-white dark:hover:bg-surface-800",
    timePicker:
      "flex items-center justify-center gap-2 border-t border-surface-200 p-2 dark:border-surface-800",
    hourPicker: "flex flex-col items-center gap-1",
    minutePicker: "flex flex-col items-center gap-1",
    secondPicker: "flex flex-col items-center gap-1",
    ampmPicker: "flex flex-col items-center gap-1",
    separator: "text-sm",
    buttonbar:
      "flex items-center justify-between border-t border-surface-200 p-2 dark:border-surface-800",
    pcIncrementButton: {
      root: cn(buttonBase, "h-7 w-7 rounded-md", buttonText.secondary),
      icon: "h-3.5 w-3.5",
    },
    pcDecrementButton: {
      root: cn(buttonBase, "h-7 w-7 rounded-md", buttonText.secondary),
      icon: "h-3.5 w-3.5",
    },
    pcTodayButton: {
      root: cn(buttonBase, buttonSize.small, buttonText.primary),
    },
    pcClearButton: {
      root: cn(buttonBase, buttonSize.small, buttonText.secondary),
    },
  },

  tabs: { root: "flex min-h-0 flex-col" },
  tablist: {
    root: "sticky top-0 z-20 shrink-0 border-b border-surface-200 bg-surface-0/95 backdrop-blur dark:border-surface-800 dark:bg-surface-950/95",
    content: "flex",
    tabList: "flex gap-1 px-1",
    activeBar: "hidden",
  },
  tab: {
    root: "-mb-px flex cursor-pointer items-center gap-1.5 border-b-2 border-transparent px-3 py-2 text-sm font-medium text-surface-500 transition-colors hover:text-surface-800 data-[p-active=true]:border-primary-500 data-[p-active=true]:text-surface-900 dark:hover:text-surface-200 dark:data-[p-active=true]:text-surface-0",
  },
  tabpanels: { root: "min-h-0 flex-1 overflow-auto pt-4" },
  tabpanel: { root: "h-full focus-visible:outline-none" },

  datatable: {
    root: "relative flex h-full flex-col overflow-hidden rounded-md border border-surface-200 bg-surface-0 text-sm dark:border-surface-800 dark:bg-surface-950",
    mask: "absolute inset-0 z-20 flex items-center justify-center bg-surface-0/70 backdrop-blur-[1px] dark:bg-surface-950/70",
    loadingIcon: "h-5 w-5 animate-spin text-primary-500",
    pcPaginator: paginator,
    tableContainer: "thin-scrollbar min-h-0 flex-1 overflow-auto",
    table: "w-max min-w-full border-collapse",
    thead:
      "sticky top-0 z-10 bg-surface-50/95 backdrop-blur dark:bg-surface-900/95",
    column: {
      headerCell:
        "border-b border-surface-200 px-4 py-2.5 text-left font-medium text-surface-500 dark:border-surface-800",
      bodyCell:
        "border-b border-surface-100 px-4 py-2 text-surface-700 dark:border-surface-800/60 dark:text-surface-200",
      columnHeaderContent: "flex items-center gap-1",
      sortIcon: "h-3.5 w-3.5 text-surface-400",
    },
    bodyRow:
      "cursor-pointer transition-colors hover:bg-surface-50 data-[p-selected=true]:bg-primary-50/70 dark:hover:bg-surface-900 dark:data-[p-selected=true]:bg-primary-500/10",
    emptyMessageCell: "px-4 py-6 text-center text-surface-400",
    pcRowCheckbox: checkbox,
    pcHeaderCheckbox: checkbox,
  },

  paginator,

  tree: {
    root: "overflow-y-auto p-2 text-sm",
    node: "",
    nodeContent:
      "flex items-center gap-1.5 rounded-md px-2 py-1.5 transition-colors hover:bg-surface-100 data-[p-selected=true]:bg-primary-50 data-[p-selected=true]:text-primary-700 dark:hover:bg-surface-800 dark:data-[p-selected=true]:bg-primary-500/10 dark:data-[p-selected=true]:text-primary-200",
    nodeToggleButton:
      "flex h-5 w-5 shrink-0 items-center justify-center rounded text-surface-400 transition-colors hover:bg-surface-200 hover:text-surface-700 data-[p-leaf=true]:invisible dark:hover:bg-surface-700 dark:hover:text-surface-100",
    nodeLabel: "flex-1 truncate text-surface-700 dark:text-surface-200",
    nodeChildren: "pl-3",
  },
};
