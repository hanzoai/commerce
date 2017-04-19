riot = require 'riot'

crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events
Api = crowdcontrol.data.Api
Page = require './page'

integrations = require '../../widget/integrations'

analyticsSnippet = '''
<!-- Hanzo analytics tag -->
<script>
  !function(t,e,a){var r,n,c,i,o,s,l,u;if(null==t.analytics){for(r=[],r.methods=["ab","alias","group","identify","off","on","once","page","pageview","ready","track","trackClick","trackForm","trackLink","trackSubmit"],l=r.methods,c=function(t){r[t]=function(){var e;return e=Array.prototype.slice.call(arguments),e.unshift(t),r.push(e),r}},i=0,o=l.length;o>i;i++)s=l[i],c(s);return u=e.createElement("script"),u.async=!0,u.type="text/javascript",u.src=a,n=e.getElementsByTagName("script")[0],n.parentNode.insertBefore(u,n),t.analytics=r}}(window,document,"//cdn.hanzo.io/a/{orgId}.js");
</script>
'''

class Integrations extends Page
  tag: 'page-integrations'
  icon: 'fa fa-credit-card'
  name: 'Integrations'
  html: require '../../templates/dash/site/pages/integrations.html'

  tab: 'analytics'

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
        if !@draggingIntegration.prototype.duplicates
          for int in @integrations[tab]
            if @draggingIntegration == int
              return

        @addIntegration @draggingIntegration, @tab
        @showSave = true

      if @draggingIntegration == integrations.Stripe
        @save()

  addIntegration: (integration, tab, model = {})->
    i = @integrations[tab].length

    if !integration.prototype.duplicates
      for int in @integrations[tab]
        if integration == int
          return

    @integrations[tab].push integration
    @models[tab].push model
    obs = {}
    riot.observable(obs)
    @obses[tab].push obs

    obs.on Events.Integration.Remove, ()=>
      obs.off Events.Integration.Remove
      console.log('remove', i)
      model = @models[tab][i]
      delete @integrations[tab][i]
      delete @models[tab][i]
      delete @obses[tab][i]

      if model.id
        @saving = true
        @api.delete("c/organization/#{window.Organization}/integrations/#{model.id}").then((res)=>
          @saving = false
          @model = res.responseText

          riot.update()
        ).catch (e)=>
          console.log(e.stack)
          @error = e

      riot.update()

    obs.on Events.Integration.Update, ()=>
      console.log('update', i)
      @showSave = true
      riot.update()

    riot.update()

  #only works for analytics right now
  save: ()->
    i = 0
    model = []

    @saving = true

    for i, m of @models['analytics']
      if m? and m._validated
        model.push m

    @api.post("c/organization/#{window.Organization}/integrations", model).then((res)=>
      if res.status != 200 && res.status != 201 && res.status != 204
        throw new Error 'Form failed to load: '

      @saving = false
      @showSave = false
      @model = res.responseText

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
        integrations.Analytics.Custom
        integrations.Analytics.FacebookConversions
        integrations.Analytics.FacebookPixel
        integrations.Analytics.GoogleAdwords
        integrations.Analytics.GoogleAnalytics
        integrations.Analytics.HeapAnalytics
        integrations.Other.Mailchimp
        integrations.Other.Mandrill
        # integrations.Other.Netlify
        integrations.Other.Reamaze
        integrations.Other.Recaptcha
        integrations.Other.Stripe
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

    @on 'update', ->
      $('.tray-right').outerHeight $('.tray-right div').outerHeight() + 100
      $('.tray-center').outerHeight $('.tray-right div').outerHeight() + 100

    $(window).on 'resize', ->
      $('.tray-right').outerHeight $('.tray-right div').outerHeight() + 100
      $('.tray-center').outerHeight $('.tray-right div').outerHeight() + 100

    @api = api = Api.get 'crowdstart'

    api.get("c/organization/#{window.Organization}/integrations").then((res)=>
      if res.status != 200 && res.status != 204
        throw new Error 'Form failed to load: '

      @model = res.responseText

      for model in @model
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
