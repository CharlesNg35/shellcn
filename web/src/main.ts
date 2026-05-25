import { createApp } from "vue";
import { createPinia } from "pinia";
import PrimeVue from "primevue/config";
import ToastService from "primevue/toastservice";
import App from "./App.vue";
import router from "./router";
import { primeVuePassthrough } from "./primevue/preset";
import "./style.css";

createApp(App)
  .use(createPinia())
  .use(router)
  .use(PrimeVue, { unstyled: true, pt: primeVuePassthrough })
  .use(ToastService)
  .mount("#app");
