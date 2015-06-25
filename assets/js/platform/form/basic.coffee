riot = require 'riot'

crowdcontrol = require 'crowdcontrol'

FormView = crowdcontrol.view.form.FormView
Api = crowdcontrol.data.Api
Source = crowdcontrol.data.Source

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
    view = @view
    view.api = api = new Api opts.url, opts.token
    view.src = src = new Source
      name: view.path + '/' + opts.userId,
      path: view.path + '/' + opts.userId,
      api: api

    src.on Source.Events.LoadData, (model)=>
      @loading = false
      @model = model

      view.loadData(model)

      view.initFormGroup.apply @
      riot.update()

  loadData: (model)->

  submit: ()->
    @api.patch(@src.path, @ctx.model)

module.exports = BasicFormView
