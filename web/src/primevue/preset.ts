// Tailwind pass-through classes for PrimeVue's unstyled components. This is the
// single place component styling lives; panels use PrimeVue components and
// inherit these classes.

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

const overlay =
  "mt-1.5 origin-top overflow-hidden rounded-lg border border-surface-200 bg-surface-0 p-1 shadow-lg ring-1 ring-surface-950/5 dark:border-surface-700 dark:bg-surface-900 dark:ring-surface-0/5";
// Rounded, inset option rows with a clear selected state in BOTH themes (the old
// selected style had no dark override → a bright light-blue bar in dark mode).
const option =
  "cursor-pointer rounded-md px-2.5 py-1.5 text-sm text-surface-700 transition-colors data-[p-focused=true]:bg-surface-100 data-[p-selected=true]:bg-primary-50 data-[p-selected=true]:font-medium data-[p-selected=true]:text-primary-700 dark:text-surface-200 dark:data-[p-focused=true]:bg-surface-800 dark:data-[p-selected=true]:bg-primary-500/15 dark:data-[p-selected=true]:text-primary-300";
// Smooth dropdown open/close — applied via each overlay component's transition pt.
const overlayTransition = {
  enterFromClass: "opacity-0 scale-95",
  enterActiveClass: "transition duration-100 ease-out",
  enterToClass: "opacity-100 scale-100",
  leaveActiveClass: "transition duration-75 ease-in",
  leaveToClass: "opacity-0",
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
    root: `inline-flex w-full items-center justify-between ${fieldSurface} text-sm transition duration-150 ${focusWithinRing}`,
    label:
      "flex-1 truncate px-2.5 py-1.5 text-left text-surface-800 dark:text-surface-100",
    dropdown: "px-2 text-surface-400",
    overlay,
    transition: overlayTransition,
    listContainer: "max-h-60 overflow-auto",
    option,
    emptyMessage: "px-3 py-2 text-sm text-surface-400",
  },

  multiselect: {
    root: `inline-flex w-full items-center justify-between ${fieldSurface} text-sm transition duration-150 ${focusWithinRing}`,
    labelContainer: "min-w-0 flex-1 overflow-hidden",
    label:
      "flex min-h-8 flex-wrap items-center gap-1 px-2.5 py-1 text-left text-surface-800 dark:text-surface-100",
    dropdown: "px-2 text-surface-400",
    overlay,
    transition: overlayTransition,
    listContainer: "max-h-60 overflow-auto",
    option,
    emptyMessage: "px-3 py-2 text-sm text-surface-400",
    chipItem:
      "rounded bg-surface-100 px-1.5 py-0.5 text-xs dark:bg-surface-800",
  },

  toggleswitch: {
    root: "relative inline-flex h-5 w-9 cursor-pointer",
    input: "absolute inset-0 z-10 cursor-pointer opacity-0",
    slider:
      "absolute inset-0 rounded-full bg-surface-300 transition-colors before:absolute before:left-0.5 before:top-0.5 before:h-4 before:w-4 before:rounded-full before:bg-white before:transition-transform data-[p-checked=true]:bg-primary-500 data-[p-checked=true]:before:translate-x-4 dark:bg-surface-700",
  },

  button: {
    root: "inline-flex items-center justify-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium transition-colors disabled:opacity-50",
  },

  dialog: {
    mask: "fixed inset-0 z-50 flex items-center justify-center bg-surface-950/50 p-4 backdrop-blur-sm",
    root: dialogRoot(),
    header:
      "flex items-center justify-between border-b border-surface-200 px-5 py-3.5 dark:border-surface-800",
    title:
      "text-base font-semibold tracking-tight text-surface-900 dark:text-surface-0",
    content: "p-5",
    footer:
      "flex items-center justify-end gap-2 border-t border-surface-200 px-5 py-3.5 dark:border-surface-800",
    pcCloseButton: {
      root: "rounded-md p-1 text-surface-400 transition-colors hover:bg-surface-100 hover:text-surface-600 dark:hover:bg-surface-800 dark:hover:text-surface-200",
    },
    // Natural rise + fade + subtle scale on open/close. The enter easing is a
    // gentle ease-out (cubic-bezier) so the dialog settles rather than snapping;
    // the global prefers-reduced-motion rule neutralizes it.
    transition: {
      enterFromClass: "opacity-0 translate-y-2 scale-[0.97]",
      enterActiveClass:
        "transition duration-200 ease-[cubic-bezier(0.16,1,0.3,1)]",
      enterToClass: "opacity-100 translate-y-0 scale-100",
      leaveFromClass: "opacity-100 translate-y-0 scale-100",
      leaveActiveClass: "transition duration-150 ease-in",
      leaveToClass: "opacity-0 translate-y-1 scale-[0.98]",
    },
  },

  tabs: { root: "flex min-h-0 flex-col" },
  tablist: {
    root: "shrink-0 border-b border-surface-200 dark:border-surface-800",
    content: "flex",
    tabList: "flex gap-1",
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
    root: "flex h-full flex-col text-sm",
    tableContainer: "min-h-0 flex-1 overflow-auto",
    table: "w-full border-collapse",
    thead: "sticky top-0 z-10 bg-surface-50 dark:bg-surface-900",
    column: {
      headerCell:
        "border-b border-surface-200 px-4 py-2 text-left font-medium text-surface-500 dark:border-surface-800",
      bodyCell:
        "border-b border-surface-100 px-4 py-1.5 text-surface-700 dark:border-surface-800/60 dark:text-surface-200",
      columnHeaderContent: "flex items-center gap-1",
      sort: "text-surface-400",
    },
    bodyRow:
      "cursor-pointer transition-colors hover:bg-surface-50 data-[p-selected=true]:bg-surface-100 dark:hover:bg-surface-900 dark:data-[p-selected=true]:bg-surface-800",
    emptyMessageCell: "px-4 py-6 text-center text-surface-400",
  },

  tree: {
    root: "overflow-y-auto p-2 text-sm",
    node: "",
    nodeContent:
      "flex items-center gap-1.5 rounded px-2 py-1 hover:bg-surface-100 data-[p-selected=true]:bg-surface-100 dark:hover:bg-surface-800 dark:data-[p-selected=true]:bg-surface-800",
    nodeToggleButton:
      "flex h-4 w-4 shrink-0 items-center justify-center text-surface-400",
    nodeLabel: "flex-1 truncate text-surface-700 dark:text-surface-200",
    nodeChildren: "pl-3",
  },
};
