crowdcontrol = require 'crowdcontrol'

View = crowdcontrol.view.View
FormView = crowdcontrol.view.form.FormView

instanceId = 0

class Integration extends FormView
  tag: 'basic-integration'
  type: 'basic-integration'
  html: ''
  instructions: 'Information on what to expect from the integration'
  img: '/img/integrations/basic.png'
  src: ''
  text: ''#'Basic Integration'
  alt: 'Basic'

  name: 'Basic Integration'

  instanceId: -1

  js: (opts)->
    super

    @model.disabled = false if !@model.disabled?

    $(@root).attr('id', 'current-integration').addClass('animated').addClass('fadeIn')

    @src = if @img then window.staticUrl + @img else ''
    @instanceId = instanceId++

  toggle: ()->
    @model.disabled = !@model.disabled

  @src: ()->
    return if @prototype.img then window.staticUrl + @prototype.img else ''

  @data: ()->
    return {
      integration: @
      src: @src()
      text: @prototype.text
      alt: @prototype.alt
    }

class IntegrationHeader extends View
  tag: 'integration-header'
  html: require '../../templates/backend/widget/integrations/header.html'

IntegrationHeader.register()

module.exports = Integration
