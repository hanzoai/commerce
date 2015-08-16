riot = require 'riot'

crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events
Page = require './page'

integrations = require '../../widget/integrations'

class Integrations extends Page
  tag: 'page-integrations'
  icon: 'fa fa-credit-card'
  name: 'Integrations'
  html: require '../../templates/backend/site/pages/integrations.html'

  tab: 'paymentprocessors'

  models:
    analytics: []

  integrationClasses:
    analytics: [
      integrations.Analytics.GoogleAnalytics
      integrations.Analytics.FacebookConversions
    ]

  integrations:
    analytics: []

  obses:
    analytics: []

  # drag events
  dragging: false
  draggingIntegration: null

  events:
    dragstart: (e, model)->
      if model?.integration?
        @dragging = true
        @draggingIntegration = model.integration
        @update()

    dragend: (e, model)->
      if model?.integration?
        return if model?.integration != @draggingIntegration

        @dragging = false
        @draggingIntegration = null

        @update()

    drop: (e)->
      if @draggingIntegration?
        i = @integrations[@tab].length

        @integrations[@tab].push @draggingIntegration
        @models[@tab].push {}
        obs = {}
        riot.observable(obs)
        @obses[@tab].push obs

        obs.on Events.Integration.Remove, ()->
          console.log('remove', i)

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
