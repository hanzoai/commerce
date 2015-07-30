riot = require 'riot'

crowdcontrol = require 'crowdcontrol'
pages = require './pages'
Router = require './router'

View = crowdcontrol.view.View

activePage = pages.Dashboard

class Crumbs extends View
  tag: 'crumbs'
  html: require '../templates/backend/site/crumbs.html'

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

Crumbs.register()

module.exports = Router.Crumbs = Crumbs
