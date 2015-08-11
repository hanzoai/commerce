crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events

View = crowdcontrol.view.View

class VariantOptionsEditor extends View
  tag: 'variant-options'
  html: require './template.html'
  events:
    "#{Events.Form.Load}": (model)->
      @formModel = model

      @model = model.options
      if !@model
        @model = model.options = []
      for option, i in @model
        option.i = i

      @update()

  addOption: ()->
    @model.push
      i: @model.length
      name: ''
      values: ''

  changeOption: (i)->
    return (event)=>
      @model[i].name = $(event.target).val()

  changeOptionValues: (i, $input)->
    @model[i].values = $input.val().split(',')

  deleteOption: (i)->
    return ()=>
      @model.splice(i, 1)
      for option, i in @model
        option.i = i

  js: ()->
    @on 'update', ()=>
      setTimeout ()=>
        $tagsInput = $(@root).find('.input-tags').each (i, el)=>
          $el = $(el)
          if !$el.parent().children('.tagsinput')[0]?
            option = @model[i]

            $el.tagsInput
              width: 'auto'
              height: 'auto'
              defaultText: '+'
              onChange: (input)=>
                @changeOptionValues(option.i, $(input))
      , 500

VariantOptionsEditor.register()
