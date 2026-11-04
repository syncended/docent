import React from "react";
import ReactDOM from "react-dom/client";
import { App } from "./App";
import { Overview } from "./Overview";
import "./style.css";

// The palette and the overview window load the same page; the overview
// window's URL ends in #overview (overview.go).
const Window = location.hash === "#overview" ? Overview : App;

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <Window />
  </React.StrictMode>,
);
