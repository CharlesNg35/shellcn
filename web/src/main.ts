import { createApp } from "vue";
import { createPinia } from "pinia";
import PrimeVue from "primevue/config";
import ToastService from "primevue/toastservice";
import ConfirmationService from "primevue/confirmationservice";
import Tooltip from "primevue/tooltip";
import App from "./App.vue";
import router from "./router";
import { primeVuePassthrough } from "./primevue/preset";
import { useTheme } from "./composables/useTheme";
import "./style.css";

// Apply the stored/system theme app-wide before first paint so every route
// (including login) renders in the right scheme with no flash.
useTheme();

createApp(App)
  .use(createPinia())
  .use(router)
  .use(PrimeVue, { unstyled: true, pt: primeVuePassthrough })
  .use(ToastService)
  .use(ConfirmationService)
  .directive("tooltip", Tooltip)
  .mount("#app");
