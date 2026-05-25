// Tailwind pass-through classes for PrimeVue's unstyled components. This is the
// single place component styling lives; panels use PrimeVue components and
// inherit these classes.
const inputBase =
  "w-full rounded-md border border-surface-300 bg-surface-0 px-2.5 py-1.5 text-sm text-surface-800 outline-none transition-colors focus:border-primary-400 dark:border-surface-700 dark:bg-surface-950 dark:text-surface-100";

const overlay =
  "mt-1 overflow-hidden rounded-md border border-surface-200 bg-surface-0 py-1 shadow-lg dark:border-surface-700 dark:bg-surface-900";
const option =
  "cursor-pointer px-3 py-1.5 text-sm text-surface-700 data-[p-focused=true]:bg-surface-100 data-[p-selected=true]:bg-primary-50 dark:text-surface-200 dark:data-[p-focused=true]:bg-surface-800";

export const primeVuePassthrough = {
  inputtext: { root: inputBase },
  textarea: { root: `${inputBase} min-h-20 font-mono` },
  inputnumber: { root: "w-full", pcInputText: { root: inputBase } },
  password: { root: "w-full", pcInputText: { root: inputBase } },

  select: {
    root: "inline-flex w-full items-center justify-between rounded-md border border-surface-300 bg-surface-0 text-sm transition-colors dark:border-surface-700 dark:bg-surface-950",
    label:
      "flex-1 truncate px-2.5 py-1.5 text-left text-surface-800 dark:text-surface-100",
    dropdown: "px-2 text-surface-400",
    overlay,
    listContainer: "max-h-60 overflow-auto",
    option,
    emptyMessage: "px-3 py-2 text-sm text-surface-400",
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
    mask: "fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4",
    root: "w-full max-w-md rounded-lg bg-surface-0 shadow-xl dark:bg-surface-900",
    header:
      "flex items-center justify-between border-b border-surface-200 px-5 py-3 dark:border-surface-800",
    title: "text-base font-semibold text-surface-900 dark:text-surface-0",
    content: "p-5",
    pcCloseButton: {
      root: "rounded p-1 text-surface-400 hover:bg-surface-100 dark:hover:bg-surface-800",
    },
  },

  tabs: { root: "shrink-0" },
  tablist: {
    root: "shrink-0 border-b border-surface-200 dark:border-surface-800",
    content: "flex",
    tabList: "flex gap-1 px-3",
  },
  tab: "flex items-center gap-1.5 border-b-2 border-transparent px-3 py-2 text-sm text-surface-500 transition-colors hover:text-surface-800 data-[p-active=true]:border-primary-500 data-[p-active=true]:text-surface-900 dark:hover:text-surface-200 dark:data-[p-active=true]:text-surface-0",
  tabpanels: { root: "min-h-0 flex-1 overflow-hidden" },
  tabpanel: { root: "h-full" },

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
