import { createStore } from "./state.js";
import { renderModel } from "./render.js";

export const store = createStore();

store.subscribe((state) => renderModel(state));
renderModel(store.getState());
