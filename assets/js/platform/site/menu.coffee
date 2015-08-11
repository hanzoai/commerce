riot = require 'riot'

crowdcontrol = require 'crowdcontrol'
pages = require './pages'
Router = require './router'

View = crowdcontrol.view.View

menu = [
  {
    name: 'Menu'
    data: [
      pages.Dashboard
      pages.Users
      pages.Orders
      pages.Payments
      pages.Products
      pages.Coupons
      pages.Stores
      pages.MailingLists
    ]
  }
  {
    name: 'System'
    data: [
      pages.Profile
      pages.Api
      pages.Organization
      pages.Integrations
    ]
  }
]

activePage = pages.Dashboard

class Menu extends View
  tag: 'menu'
  html: require '../templates/backend/site/menu.html'
  model: menu

  route: (url)->
    riot.route(url)

  js: ()->
    super

    requestAnimationFrame ()->
      try
        window?.Core?.init()
      catch e
        e
        #console?.log e

  setActive: (p)->
    activePage = p
    @update()

  getActive: ()->
    return activePage

  @setActive: (p)->
    activePage = p
    riot.update()

  @getActive:()->
    return activePage

Menu.register()

module.exports = Router.Menu = Menu
