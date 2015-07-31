riot = require 'riot'

crowdcontrol = require 'crowdcontrol'

FormView = crowdcontrol.view.form.FormView
Api = crowdcontrol.data.Api
Source = crowdcontrol.data.Source
m = crowdcontrol.utils.mediator

LoadEvent = 'Load'

class BasicFormView extends FormView
  @Events:
    Load: LoadEvent
  tag: 'basic-form'
  redirectPath: ''
  path: ''
  html: require './template.html'
  id: null

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
    @api.delete(@path).finally ()=>
      window.location.hash = @redirectPath

  js: (opts)->
    super

    if @id?
      @loading = true
      m.trigger 'start-spin', @path + '-form-load'

      @api = api = Api.get('crowdstart')
      api.get(@path).then((res)=>
        m.trigger 'stop-spin', @path + '-form-load'

        if res.status != 200
          throw new Error("Form failed to load")

        @model = res.responseText
        @loadData @model

        @initFormGroup()

        @obs.trigger LoadEvent, @model
        riot.update()
      ).catch ()=>
        window.location.hash = @redirectPath

  loadData: (model)->

  _submit: (event)->
    m.trigger 'start-spin', @path + '-form-save'
    @update()

    method = if @id? then 'patch' else 'post'

    return @api[method](@path, @model).then ()=>
      m.trigger 'stop-spin', @path + '-form-save'
      $button = $(event.target).find('input[type=submit], button[type=submit]').text('Saved')
      setTimeout ()->
        $button.text('Save')
      , 1000
      @update()
    , ()=>
      m.trigger 'stop-spin', @path + '-form-save'
      $button = $(event.target).find('input[type=submit], button[type=submit]').text('An Error Has Occured')
      setTimeout ()->
        $button.text('Save')
      , 1000
      @update()

module.exports = BasicFormView
