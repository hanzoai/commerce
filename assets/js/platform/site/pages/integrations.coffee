riot = require 'riot'

crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events
Api = crowdcontrol.data.Api
Page = require './page'

integrations = require '../../widget/integrations'

class Integrations extends Page
  tag: 'page-integrations'
  icon: 'fa fa-credit-card'
  name: 'Integrations'
  html: require '../../templates/backend/site/pages/integrations.html'

  tab: 'paymentprocessors'

  # models maintained by integration models
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
        @addIntegration @draggingIntegration, @tab

  addIntegration: (integration, tab, model = {})->
    i = @integrations[tab].length

    @integrations[tab].push integration
    @models[tab].push model
    obs = {}
    riot.observable(obs)
    @obses[tab].push obs

    obs.on Events.Integration.Remove, ()=>
      obs.off Events.Integration.Remove
      console.log('remove', i)
      delete @integrations[tab][i]
      delete @models[tab][i]
      delete @obses[tab][i]
      riot.update()

    riot.update()

  setType: (t)->
    return (e)=>
      @tab = t
      e.preventDefault()

  collection: 'integrations'

  isTabEmpty: (tab)->
    for integration in @integrations[tab]
      return false if integration
    return true

  js: ()->
    super

    @on 'update', ()->
      $('#current-page').css
        'padding-bottom': '20px'

    # models maintained by integration models
    @models =
      analytics: []

    @integrationClasses =
      analytics: [
        integrations.Analytics.GoogleAnalytics
        integrations.Analytics.FacebookConversions
      ]

    @integrations =
      analytics: []

    @obses =
      analytics: []

    requestAnimationFrame ()->
      try
        # needs to init twice to cancel soem things
        window?.Core?.init()
        window?.Core?.init()
      catch e
        e
        #console?.log e

    @api = api = Api.get 'crowdstart'

    api.get("c/organization/#{window.Organization}/analytics").then((res)=>
      if res.status != 200 && res.status != 204
        throw new Error 'Form failed to load: '

      @model = res.responseText

      for model in @model.integrations
        for analyticsClass in @integrationClasses.analytics
          if model.type == analyticsClass.prototype.type
            @addIntegration analyticsClass, 'analytics', model
            break

      riot.update()
    ).catch (e)=>
      console.log(e.stack)
      @error = e

Integrations.register()

riot.tag 'integration', '', (opts)->
  type = opts.type

  if !type?
    return

  riot.mount @root, type.prototype.tag, opts

module.exports = Integrations
