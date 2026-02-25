import { DashboardApp } from "./dashboard-app"
import { DashboardPlugin } from "./dashboard-app/types"

import displayModule from "./lib/extensions/displays"
import formModule from "./lib/extensions/forms"
import i18nModule from "./lib/extensions/i18n"
import menuItemModule from "./lib/extensions/menu-items"
import routeModule from "./lib/extensions/routes"
import widgetModule from "./lib/extensions/widgets"

import "./index.css"

const localPlugin: DashboardPlugin = {
  widgetModule,
  routeModule,
  displayModule,
  formModule,
  menuItemModule,
  i18nModule,
}

interface AppProps {
  plugins?: DashboardPlugin[]
}

function App({ plugins = [] }: AppProps) {
  const app = new DashboardApp({
    plugins: [localPlugin, ...plugins],
  })

  return <div>{app.render()}</div>
}

export default App
