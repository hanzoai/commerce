riot = require 'riot'

Page = require './page'

integrations = require '../../widget/integrations'

class Integrations extends Page
  tag: 'page-integrations'
  icon: 'fa fa-credit-card'
  name: 'Integrations'
  html: require '../../templates/backend/site/pages/integrations.html'

  tab: 'paymentprocessors'

  iModels:
    analytics: []

  models:
    analytics: []

  integrations:
    analytics: [
      integrations.Analytics.GoogleAnalytics
      integrations.Analytics.FacebookConversions
    ]

  # drag events
  dragging: false
  draggingModel: null

  events:
    dragstart: (e, model)->
      if model?.integration?
        @dragging = true
        @integrationModel = model
        @update()

    dragend: (e, model)->
      if model?.integration?
        return if model != @integrationModel

        @dragging = false
        @integrationModel = null

        @update()

    drop: (e)->
      if @integrationModel?.integration?
        @iModels[@tab].push @integrationModel
        @models[@tab].push {}

        @update()

  setType: (t)->
    return (e)=>
      @tab = t
      e.preventDefault()

  collection: 'integrations'

  js: ()->
    super

    @on 'update', ()->
      $('#current-page').css
        'padding-bottom': '20px'

    requestAnimationFrame ()->
      try
        window?.Core?.init()
      catch e
        e
        #console?.log e

Integrations.register()

riot.tag 'integration', '', (opts)->
  type = opts.type

  riot.mount @root, type.prototype.tag, opts

module.exports = Integrations
