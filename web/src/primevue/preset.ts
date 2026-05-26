// Tailwind pass-through classes for PrimeVue's unstyled components. This is the
// single place component styling lives; panels use PrimeVue components and
// inherit these classes.

import type { ButtonPassThroughMethodOptions } from "primevue/button";

// Shared field building blocks — composed below and re-exported so hand-rolled
// inputs (search boxes, etc.) reuse the exact same look instead of duplicating it.
export const fieldSurface =
  "rounded-md border border-surface-300 bg-surface-0 dark:border-surface-700 dark:bg-surface-950";
const focusRing =
  "focus:border-primary-500 focus:ring-2 focus:ring-primary-500/30";
const focusWithinRing =
  "focus-within:border-primary-500 focus-within:ring-2 focus-within:ring-primary-500/30";

const inputBase = `w-full ${fieldSurface} px-2.5 py-1.5 text-sm text-surface-800 outline-none transition duration-150 placeholder:text-surface-400 ${focusRing} dark:text-surface-100`;

// A standalone text input matching the PrimeVue inputs (for plain <input>s).
export const inputClass = inputBase;

// A search box with room for a leading icon — shared by the sidebar and pickers.
export const searchInputClass = `w-full ${fieldSurface} py-1.5 pl-9 pr-3 text-sm text-surface-800 outline-none transition duration-150 placeholder:text-surface-400 ${focusRing} dark:text-surface-100`;

// The dialog box surface — single source for every modal so width is the only
// per-dialog difference (avoids repeating the box classes in each component).
export const dialogRoot = (maxWidth = "max-w-md"): string =>
  `w-full ${maxWidth} overflow-hidden rounded-xl border border-surface-200 bg-surface-0 shadow-2xl ring-1 ring-surface-950/5 dark:border-surface-800 dark:bg-surface-900 dark:ring-surface-0/5`;

// Shared button looks, reused across dialogs/action bars instead of re-listing
// the same utility chains.
export const btnPrimary =
  "inline-flex items-center justify-center gap-1.5 rounded-md bg-primary-600 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-primary-700 disabled:opacity-50";
export const btnGhost =
  "inline-flex items-center justify-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium text-surface-600 transition-colors hover:bg-surface-100 disabled:opacity-50 dark:text-surface-300 dark:hover:bg-surface-800";

const buttonBase =
  "inline-flex items-center justify-center gap-1.5 whitespace-nowrap rounded-md text-sm font-medium outline-none transition-colors focus-visible:ring-2 focus-visible:ring-primary-500/40 disabled:pointer-events-none disabled:opacity-50";
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
  danger: "bg-red-600 text-white hover:bg-red-700",
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
    "text-red-700 hover:bg-red-50 dark:text-red-300 dark:hover:bg-red-500/10",
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
    "border border-red-300 text-red-700 hover:bg-red-50 dark:border-red-700 dark:text-red-300 dark:hover:bg-red-500/10",
  contrast:
    "border border-surface-600 text-surface-900 hover:bg-surface-100 dark:border-surface-400 dark:text-surface-100 dark:hover:bg-surface-800",
};

type ButtonTone = keyof typeof buttonSolid;

const buttonTones = [
  "secondary",
  "success",
  "info",
  "warn",
  "help",
  "danger",
  "contrast",
] as const;

function buttonTone(
  severity: ButtonPassThroughMethodOptions<unknown>["props"]["severity"],
): ButtonTone {
  return (buttonTones as readonly string[]).includes(severity ?? "")
    ? (severity as ButtonTone)
    : "primary";
}

function buttonRoot(options: ButtonPassThroughMethodOptions<unknown>): string {
  const props = options.props;
  const tone = buttonTone(props.severity);
  const size =
    props.size === "small"
      ? buttonSize.small
      : props.size === "large"
        ? buttonSize.large
        : buttonSize.normal;
  const shape = props.rounded ? "rounded-full" : "rounded-md";
  const width = props.fluid ? "w-full" : "";
  const variant = props.variant;
  if (props.link || variant === "link") {
    return `${buttonBase} ${shape} ${width} px-0 py-0 text-primary-700 hover:text-primary-800 hover:underline dark:text-primary-300 dark:hover:text-primary-200`;
  }
  if (props.text || variant === "text") {
    return `${buttonBase} ${shape} ${width} ${size} ${buttonText[tone]}`;
  }
  if (props.outlined || variant === "outlined") {
    return `${buttonBase} ${shape} ${width} ${size} ${buttonOutlined[tone]}`;
  }
  return `${buttonBase} ${shape} ${width} ${size} ${buttonSolid[tone]}`;
}

const overlay =
  "mt-1.5 origin-top overflow-hidden rounded-lg border border-surface-200 bg-surface-0 p-1 shadow-lg ring-1 ring-surface-950/5 dark:border-surface-700 dark:bg-surface-900 dark:ring-surface-0/5";
// Rounded, inset option rows with a clear selected state in BOTH themes (the old
// selected style had no dark override → a bright light-blue bar in dark mode).
const option =
  "cursor-pointer truncate rounded-md px-2.5 py-1.5 text-sm text-surface-700 transition-colors data-[p-focused=true]:bg-surface-100 data-[p-selected=true]:bg-primary-50 data-[p-selected=true]:font-medium data-[p-selected=true]:text-primary-700 dark:text-surface-200 dark:data-[p-focused=true]:bg-surface-800 dark:data-[p-selected=true]:bg-primary-500/15 dark:data-[p-selected=true]:text-primary-300";
// Smooth dropdown open/close — applied via each overlay component's transition pt.
const overlayTransition = {
  enterFromClass: "opacity-0 scale-95",
  enterActiveClass: "transition duration-100 ease-out",
  enterToClass: "opacity-100 scale-100",
  leaveActiveClass: "transition duration-75 ease-in",
  leaveToClass: "opacity-0",
};

// Shared dialog chrome, reused by both Dialog and ConfirmDialog (which render the
// same header/footer/transition). Mask carries no z-index so each consumer can set
// its own stacking order.
const dialogMask =
  "fixed inset-0 flex items-center justify-center bg-surface-950/50 p-4 backdrop-blur-sm";
const dialogHeader =
  "flex items-center justify-between border-b border-surface-200 px-5 py-3.5 dark:border-surface-800";
const dialogTitle =
  "text-base font-semibold tracking-tight text-surface-900 dark:text-surface-0";
const dialogFooter =
  "flex items-center justify-end gap-2 border-t border-surface-200 px-5 py-3.5 dark:border-surface-800";
// Natural rise + fade + subtle scale on open/close; the global prefers-reduced-motion
// rule neutralizes it.
const dialogTransition = {
  enterFromClass: "opacity-0 translate-y-2 scale-[0.97]",
  enterActiveClass: "transition duration-200 ease-[cubic-bezier(0.16,1,0.3,1)]",
  enterToClass: "opacity-100 translate-y-0 scale-100",
  leaveFromClass: "opacity-100 translate-y-0 scale-100",
  leaveActiveClass: "transition duration-150 ease-in",
  leaveToClass: "opacity-0 translate-y-1 scale-[0.98]",
};

// Shared checkbox visuals (standalone Checkbox + MultiSelect option/header checks).
const checkbox = {
  root: "relative inline-flex h-4 w-4 shrink-0",
  input: "absolute inset-0 cursor-pointer opacity-0",
  box: "flex h-4 w-4 items-center justify-center rounded border border-surface-300 bg-surface-0 transition-colors dark:border-surface-600 dark:bg-surface-950 data-[p-checked=true]:border-primary-500 data-[p-checked=true]:bg-primary-500 data-[p-checked=true]:text-white",
  icon: "h-3 w-3 text-white",
};

const paginatorButton =
  "inline-flex h-8 min-w-8 items-center justify-center rounded-md px-2 text-sm text-surface-600 transition-colors hover:bg-surface-100 disabled:pointer-events-none disabled:opacity-40 data-[p-selected=true]:bg-primary-50 data-[p-selected=true]:font-medium data-[p-selected=true]:text-primary-700 dark:text-surface-300 dark:hover:bg-surface-800 dark:data-[p-selected=true]:bg-primary-500/15 dark:data-[p-selected=true]:text-primary-300";
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
    root: `flex min-w-20 items-center justify-between ${fieldSurface} text-sm transition duration-150 ${focusWithinRing}`,
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
  textarea: { root: `${inputBase} min-h-20 font-mono` },
  inputnumber: { root: "w-full", pcInputText: { root: inputBase } },
  password: {
    root: "relative block w-full",
    // Leave room on the right for the absolutely-positioned show/hide toggle.
    pcInputText: { root: `${inputBase} pr-9` },
    maskIcon:
      "absolute right-3 top-1/2 -translate-y-1/2 cursor-pointer text-surface-400 transition-colors hover:text-surface-600 dark:hover:text-surface-300",
    unmaskIcon:
      "absolute right-3 top-1/2 -translate-y-1/2 cursor-pointer text-surface-400 transition-colors hover:text-surface-600 dark:hover:text-surface-300",
  },

  select: {
    // min-w-0 lets the flex-1 label actually shrink so `truncate` can ellipsize
    // a long selected value instead of overflowing/pushing the dropdown icon out.
    root: `flex w-full min-w-0 items-center justify-between ${fieldSurface} text-sm transition duration-150 ${focusWithinRing}`,
    label:
      "min-w-0 flex-1 truncate px-2.5 py-1.5 text-left text-surface-800 dark:text-surface-100",
    dropdown: "shrink-0 px-2 text-surface-400",
    overlay,
    transition: overlayTransition,
    listContainer: "max-h-60 overflow-auto p-1",
    option,
    emptyMessage: "px-3 py-2 text-sm text-surface-400",
  },

  // Standalone Checkbox (also reused inside MultiSelect rows).
  checkbox,
  multiselect: {
    root: `flex w-full items-center justify-between ${fieldSurface} text-sm transition duration-150 ${focusWithinRing}`,
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
    // MultiSelect options are a checkbox + label row, so they need inline flex
    // layout (the plain Select `option` style stacked them).
    option:
      "flex cursor-pointer items-center gap-2.5 rounded-md px-2.5 py-1.5 text-sm text-surface-700 transition-colors data-[p-focused=true]:bg-surface-100 dark:text-surface-200 dark:data-[p-focused=true]:bg-surface-800",
    optionLabel: "min-w-0 flex-1 truncate",
    pcOptionCheckbox: checkbox,
    emptyMessage: "px-3 py-2 text-sm text-surface-400",
    // Selected values render as inline chips (label + remove icon side by side).
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
      root: `${buttonBase} ${buttonSize.normal} ${buttonSolid.secondary} cursor-pointer`,
    },
  },

  progressbar: {
    root: "relative overflow-hidden rounded-full bg-surface-200 dark:bg-surface-800",
    value:
      "h-full rounded-full bg-primary-500 transition-[width] duration-150 data-[p-progressbar-value=true]:bg-primary-500",
    label: "hidden",
  },

  toggleswitch: {
    root: "relative inline-flex h-5 w-9 cursor-pointer",
    input: "absolute inset-0 z-10 cursor-pointer opacity-0",
    slider:
      "absolute inset-0 rounded-full bg-surface-300 transition-colors before:absolute before:left-0.5 before:top-0.5 before:h-4 before:w-4 before:rounded-full before:bg-white before:transition-transform data-[p-checked=true]:bg-primary-500 data-[p-checked=true]:before:translate-x-4 dark:bg-surface-700",
  },

  button: {
    root: buttonRoot,
    icon: "h-4 w-4 shrink-0",
    loadingIcon: "h-4 w-4 shrink-0 animate-spin",
    label: "truncate",
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
    mask: `${dialogMask} z-50`,
    root: dialogRoot(),
    header: dialogHeader,
    title: dialogTitle,
    content: "p-5",
    footer: dialogFooter,
    pcCloseButton: {
      root: "rounded-md p-1 text-surface-400 transition-colors hover:bg-surface-100 hover:text-surface-600 dark:hover:bg-surface-800 dark:hover:text-surface-200",
    },
    transition: dialogTransition,
  },

  // Shares the dialog's chrome; mask sits above open dialogs (z-60) so a confirm
  // raised from within a dialog (e.g. revoke inside Share) is never occluded.
  confirmdialog: {
    mask: `${dialogMask} z-[60]`,
    root: dialogRoot("max-w-md"),
    header: dialogHeader,
    title: dialogTitle,
    content: "px-5 py-5",
    footer: dialogFooter,
    transition: dialogTransition,
  },

  popover: {
    root: "z-50 mt-1.5 rounded-lg border border-surface-200 bg-surface-0 shadow-lg ring-1 ring-surface-950/5 dark:border-surface-700 dark:bg-surface-900 dark:ring-surface-0/5",
    content: "p-3",
    transition: overlayTransition,
  },

  tabs: { root: "flex min-h-0 flex-col" },
  tablist: {
    root: "shrink-0 border-b border-surface-200 dark:border-surface-800",
    content: "flex",
    tabList: "flex gap-1 px-1",
    // We indicate the active tab with a per-tab underline, so hide PrimeVue's
    // sliding active bar (it would render unstyled in unstyled mode).
    activeBar: "hidden",
  },
  // Object form (not a bare string): a string under a global pt component key is
  // ignored, which left tabs completely unstyled.
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
    tableContainer: "min-h-0 flex-1 overflow-auto",
    table: "w-full border-collapse",
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
