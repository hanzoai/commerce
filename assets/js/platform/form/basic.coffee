riot = require 'riot'

crowdcontrol = require 'crowdcontrol'

FormView = crowdcontrol.view.form.FormView
Api = crowdcontrol.data.Api
Source = crowdcontrol.data.Source
m = crowdcontrol.utils.mediator

class BasicFormView extends FormView
  tag: 'basic-form'
  path: ''
  html: ''
  events:
    "#{FormView.Events.SubmitFailed}": ()->
      requestAnimationFrame ()->
        $container = $(".error-container")
        if $container[0]
          $('html, body').animate(
            scrollTop: $container.offset().top-$(window).height()/2
          , 1000)
  js: (opts)->
    super

    #case sensitivity issues
    opts.userId = opts.userId || opts.userid

    @loading = true
    m.trigger 'start-spin', 'user-form-load'

    @api = api = new Api opts.url, opts.token
    @src = src = new Source
      name: @path + '/' + opts.userId,
      path: @path + '/' + opts.userId,
      api: api

    src.on Source.Events.LoadData, (model)=>
      m.trigger 'stop-spin', 'user-form-load'
      @model = model

      @loadData(model)

      @initFormGroup()
      riot.update()

  loadData: (model)->

  _submit: (event)->
    m.trigger 'start-spin', 'user-form-save'
    @update()

    return @api.patch(@src.path, @model).then ()=>
      m.trigger 'stop-spin', 'user-form-save'
      $button = $(event.target).find('input[type=submit], button[type=submit]').text('Saved')
      setTimeout ()->
        $button.text('Save')
      , 1000
      @update()
    , ()=>
      m.trigger 'stop-spin', 'user-form-save'
      $button = $(event.target).find('input[type=submit], button[type=submit]').text('Saved')
      setTimeout ()->
        $button.text('Save')
      , 1000
      @update()

module.exports = BasicFormView
