import { createApp } from "vue";
import { createPinia } from "pinia";
import "element-plus/es/components/message/style/css";
import "element-plus/es/components/message-box/style/css";

import App from "./App.vue";
import router from "./router";
import "./styles.css";
import { appTitle } from "./config/branding";

const app = createApp(App);
app.use(createPinia());
app.use(router);
if (typeof document !== "undefined") {
  document.title = appTitle;
}
app.mount("#app");
