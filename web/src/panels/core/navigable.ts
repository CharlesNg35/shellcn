import { inject, provide, ref, type InjectionKey, type Ref } from "vue";

// The set of resource kinds a connection can navigate to (those with a detail
// view). Provided by the workspace from the manifest projection; the generic
// table uses it to decide row-click = navigate vs select, without any per-table
// declaration. Empty when no provider (e.g. standalone tests).
const NAVIGABLE_KINDS: InjectionKey<Ref<ReadonlySet<string>>> =
  Symbol("navigable-kinds");

export function provideNavigableKinds(kinds: Ref<ReadonlySet<string>>): void {
  provide(NAVIGABLE_KINDS, kinds);
}

export function useNavigableKinds(): Ref<ReadonlySet<string>> {
  return inject(NAVIGABLE_KINDS, ref(new Set<string>()));
}
