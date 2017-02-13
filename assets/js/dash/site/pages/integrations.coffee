riot = require 'riot'

crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events
Api = crowdcontrol.data.Api
Page = require './page'

integrations = require '../../widget/integrations'

analyticsSnippet = '''
<!-- Crowdstart analytics tag -->
<script>
  !function(t,e,a){var r,n,c,i,o,s,l,u;if(null==t.analytics){for(r=[],r.methods=["ab","alias","group","identify","off","on","once","page","pageview","ready","track","trackClick","trackForm","trackLink","trackSubmit"],l=r.methods,c=function(t){r[t]=function(){var e;return e=Array.prototype.slice.call(arguments),e.unshift(t),r.push(e),r}},i=0,o=l.length;o>i;i++)s=l[i],c(s);return u=e.createElement("script"),u.async=!0,u.type="text/javascript",u.src=a,n=e.getElementsByTagName("script")[0],n.parentNode.insertBefore(u,n),t.analytics=r}}(window,document,"//cdn.hanzo.io/a/{orgId}.js");
</script>
'''

class Integrations extends Page
  tag: 'page-integrations'
  icon: 'fa fa-credit-card'
  name: 'Integrations'
  html: require '../../templates/backend/site/pages/integrations.html'

  tab: 'paymentprocessors'

  # models maintained by integration models
  # models:
  #   analytics: []

  # integrationClasses:
  #   analytics: [
  #     integrations.Analytics.GoogleAnalytics
  #     integrations.Analytics.FacebookConversions
  #   ]

  # integrations:
  #   analytics: []

  # obses:
  #   analytics: []

  # drag flags
  dragging: false
  draggingIntegration: null

  # save flags
  showSave: false
  saving: false

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

    obs.on Events.Integration.Update, ()=>
      console.log('update', i)
      @showSave = true
      riot.update()

    riot.update()

  #only works for analytics right now
  save: (event)->
    model =
      integrations: []

    for m in @models['analytics']
      if m? and m._validated
        model.integrations.push(m)

    @saving = true

    @api.post("c/organization/#{window.Organization}/analytics", model).then((res)=>
      @saving = false
      @showSave = false
      @model = model

      if res.status != 200 && res.status != 201 && res.status != 204
        throw new Error 'Form failed to load: '

      riot.update()
    ).catch (e)=>
      console.log(e.stack)
      @error = e

  setType: (t)->
    return (e)=>
      @tab = t
      e.preventDefault()
      riot.update()

  collection: 'integrations'

  isTabEmpty: (tab)->
    for integration in @integrations[tab]
      return false if integration
    return true

  js: ()->
    super

    @analyticsSnippet = analyticsSnippet.replace '{orgId}', window.Organization

    #set up model defaults
    @models =
      analytics: []

    @integrationClasses =
      analytics: [
        integrations.Analytics.GoogleAnalytics
        integrations.Analytics.GoogleAdwords
        integrations.Analytics.FacebookConversions
        integrations.Analytics.FacebookPixel
        integrations.Analytics.Custom
        integrations.Analytics.HeapAnalytics
      ]

    @integrations =
      analytics: []

    @obses =
      analytics: []

    requestAnimationFrame ()->
      try
        # needs to init twice so that side bar works things
        window?.Core?.init()
        window?.Core?.init()
      catch e
        e
        #console?.log e

    @on 'update', ()->
      $('.tray-right').outerHeight $('#content').outerHeight()

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
