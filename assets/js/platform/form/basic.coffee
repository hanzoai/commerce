riot = require 'riot'

crowdcontrol = require 'crowdcontrol'

FormView = crowdcontrol.view.form.FormView
Api = crowdcontrol.data.Api
Source = crowdcontrol.data.Source
m = crowdcontrol.utils.mediator

SuccessEvent = 'form-submit-success'
LoadEvent = 'form-data-load'

class BasicFormView extends FormView
  @Events:
    Load: LoadEvent
    Success: SuccessEvent
  tag: 'basic-form'
  redirectPath: ''
  path: ''
  html: require './template.html'
  id: null
  error: null

  events:
    "#{FormView.Events.SubmitFailed}": ()->
      requestAnimationFrame ()->
        $container = $(".error-container")
        if $container[0]
          $('html, body').animate(
            scrollTop: $container.offset().top-$(window).height()/2
          , 1000)

  delete: ()->
    m.trigger 'start-spin', @path + '-delete'
    @api.delete(@path).finally (e)=>
      console.log(e.stack) if e
      window.location.hash = @redirectPath

  js: (opts)->
    super

    @api = api = Api.get 'crowdstart'

    if @id?
      @loading = true
      m.trigger 'start-spin', @path + '-form-load'

      api.get(@path).then((res)=>
        m.trigger 'stop-spin', @path + '-form-load'

        if res.status != 200 && res.status != 204
          throw new Error 'Form failed to load: '

        @model = res.responseText
        @loadData @model

        @initFormGroup()

        @obs.trigger LoadEvent, @model
        riot.update()
      ).catch (e)=>
        console.log(e.stack)
        window.location.hash = @redirectPath
    else
      # the LoadEvent is meant to be triggered asynchrous of the object bootstrapping
      # otherwise, it will fire before riot.mount finishes rendering this tag's children
      requestAnimationFrame ()=>
        @obs.trigger LoadEvent, @model

  initFormGroup: ()->
    if !@id? && @inputs?
      for key, input of @inputs
        input.model.value = ''

    super

  loadData: (model)->

  _submit: (event)->
    m.trigger 'start-spin', @path + '-form-save'
    @update()

    method = if @id? then 'patch' else 'post'

    $button = $(event.target).find('input[type=submit], button[type=submit]')
    buttonText= $button.text()
    $button.text 'Processing'
    $button.prop 'disabled', true
    @fullyValidated = false

    return @api[method](@path, @model).then((res)=>
      if res.status != 200 && res.status != 204
        throw new Error res.responseText.error.message

      @error = undefined

      m.trigger 'stop-spin', @path + '-form-save'
      $button.text 'Saved'

      setTimeout ()->
        $button.text buttonText
        $button.prop 'disabled', false
      , 1000
      @obs.trigger SuccessEvent
      @update()
    ).catch (e)=>
      @error = e

      m.trigger 'stop-spin', @path + '-form-save'
      $button.text 'An Error Has Occured'

      setTimeout ()->
        $button.text buttonText
        $button.prop 'disabled', false
      , 1000
      @update()

module.exports = BasicFormView
