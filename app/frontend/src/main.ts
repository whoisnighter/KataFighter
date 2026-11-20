import { mount } from "svelte";
import App from "./App.svelte";
import logo from "../../../docs/assets/logo.svg";

// Favicon from the single logo source (docs/assets/logo.svg).
document.querySelector<HTMLLinkElement>('link[rel="icon"]')!.href = logo;

mount(App, { target: document.getElementById("app")! });
