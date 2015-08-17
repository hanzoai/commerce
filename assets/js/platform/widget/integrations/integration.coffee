crowdcontrol = require 'crowdcontrol'

View = crowdcontrol.view.View
Events = crowdcontrol.Events
FormView = crowdcontrol.view.form.FormView

instanceId = 0

Events.Integration =
  Save: 'integration-save'
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
  realSubmit: false

  events:
    "#{ Events.Form.SubmitFailed }": ()->
      @error = true
      @realSubmit = false
      @update()

    "#{ Events.Form.SubmitSuccess }": ()->
      @error = false
      @update()

    "#{ Events.Input.Error }": ()->
      @error = true
      @update()

    "#{ Events.Input.Set }": ()->
      @submit()
      @update()

  save: ()->
    @realSubmit = true
    @submit()

  _submit: ()->
    if @realSubmit
      @obs.trigger Events.Integration.Save
      @realSubmit = false

  js: (opts)->
    super

    @on 'update', ()->
      $('[data-toggle="tooltip"]').tooltip()

    @model.disabled = false if !@model.disabled?

    $(@root).attr('id', 'current-integration').addClass('animated').addClass('fadeIn')

    @src = if @img then window.staticUrl + @img else ''
    @instanceId = instanceId++

    requestAnimationFrame ()=>
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

  remove: ()->
    @obs.trigger Events.Integration.Remove

  toggle: ()->
    @model.disabled = !@model.disabled
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
  html: require '../../templates/backend/widget/integrations/header.html'

IntegrationHeader.register()

module.exports = Integration
