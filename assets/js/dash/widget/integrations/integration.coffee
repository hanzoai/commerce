crowdcontrol = require 'crowdcontrol'

View = crowdcontrol.view.View
Events = crowdcontrol.Events
FormView = crowdcontrol.view.form.FormView

instanceId = 0

Events.Integration =
  Update: 'integration-updated'
  Remove: 'integration-close'

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

  error: false
  fakeSubmit: true

  events:
    "#{ Events.Form.SubmitFailed }": ()->
      @error = true
      @fakeSubmit = false
      @model._validated = false
      @update()

    "#{ Events.Form.SubmitSuccess }": ()->
      @error = false
      @model._validated = true
      @update()

    "#{ Events.Input.Error }": ()->
      @error = true
      @update()

    "#{ Events.Input.Set }": ()->
      @submit()
      @update()

  _submit: ()->
    if @fakeSubmit
      @fakeSubmit = false
      return

    @obs.trigger Events.Integration.Update

  js: (opts)->
    super

    @on 'update', ()->
      $('[data-toggle="tooltip"]').tooltip()

    @model.disabled = false if !@model.disabled?
    @model.type = @type

    $(@root).attr('id', 'current-integration').addClass('animated').addClass('fadeIn')

    @src = if @img then window.staticUrl + @img else ''
    @instanceId = instanceId++

    requestAnimationFrame ()=>
      @fakeSubmit = true
      @submit()

    @update()

  removeModal: ()->
    bootbox.dialog
      title: 'Are You Sure?'
      message: 'Removing this integration will delete its settings.'

      buttons:
        Yes:
          className: 'btn btn-danger'
          callback: ()=>
            @remove()

        No:
          className: 'btn btn-primary'
          callback: ()->

  remove: (event)->
    @obs.trigger Events.Integration.Remove, event

  toggle: (event)->
    @model.disabled = !@model.disabled
    @submit()
    @update()

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
  html: require '../../templates/dash/widget/integrations/header.html'

IntegrationHeader.register()

module.exports = Integration
