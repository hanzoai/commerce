'use client'

import { DashboardApp } from "../dashboard-app"
import { DashboardPlugin } from "../dashboard-app/types"

import displayModule from "../lib/extensions/displays"
import formModule from "../lib/extensions/forms"
import i18nModule from "../lib/extensions/i18n"
import menuItemModule from "../lib/extensions/menu-items"
import routeModule from "../lib/extensions/routes"
import widgetModule from "../lib/extensions/widgets"

const localPlugin: DashboardPlugin = {
  widgetModule,
  routeModule,
  displayModule,
  formModule,
  menuItemModule,
  i18nModule,
}

interface AdminAppProps {
  plugins?: DashboardPlugin[]
}

export function AdminApp({ plugins = [] }: AdminAppProps) {
  const app = new DashboardApp({
    plugins: [localPlugin, ...plugins],
  })

  return <div>{app.render()}</div>
}
