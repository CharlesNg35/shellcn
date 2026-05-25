import { createApp } from "vue";
import { createPinia } from "pinia";
import PrimeVue from "primevue/config";
import App from "./App.vue";
import "./style.css";

createApp(App)
  .use(createPinia())
  .use(PrimeVue, { unstyled: true })
  .mount("#app");
