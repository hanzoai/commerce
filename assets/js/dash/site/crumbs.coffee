riot = require 'riot'

crowdcontrol = require 'crowdcontrol'
pages = require './pages'
Router = require './router'

View = crowdcontrol.view.View

activePage = pages.Dashboard
_id = ''

class Crumbs extends View
  tag: 'crumbs'
  html: require '../templates/dash/site/crumbs.html'

  js: ()->
    super

    requestAnimationFrame ()->
      try
        window?.Core?.init()
      catch e
        e
        #console?.log e

  setActiveId: (id)->
    _id = id

  getActiveId: ()->
    return _id

  @setActiveId: (id)->
    _id = id

  @getActiveId: ()->
    return _id

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
