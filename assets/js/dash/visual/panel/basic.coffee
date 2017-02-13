crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events
View = crowdcontrol.view.View
humanize = require 'humanize'

spinFrames = 10

class BasicPanelView extends View
  tag: 'basic-panel'
  label: ''
  description: 'Description'
  html: require '../../templates/dash/visual/panel/template.html'
  events:
    "#{ Events.Visual.NewData }": ()->
      @loadData.apply @, arguments
      @update()

    "#{ Events.Visual.NewDescription }": (description)->
      @description = description
      @update()

    "#{ Events.Visual.NewLabel }": (label)->
      @label = label
      @update()

  js: (opts)->
    @model = 0
    @label = opts.label ? @label
    @description = opts.description ? @description
    @loadData @model, opts.compareModel

  canCompare: ()->
    @compareModel != 0 && !isNaN @compareModel

  loadData: (model, compareModel)->
    @model = model ? 0
    @compareModel = compareModel ? NaN
    if @canCompare()
      @comparePercent = ((@model - @compareModel) / @compareModel * 100).toFixed(1)

    @spinNumber = 0
    if @model == 0
      @update()
      return

    spinActualNumber = 0
    deltaSpin = @model / spinFrames
    frames = spinFrames
    spin = ()=>
      requestAnimationFrame ()=>
        spinActualNumber += deltaSpin
        @spinNumber = parseInt spinActualNumber, 10
        @update()

        if frames > 1
          frames--
          spin()
    spin()

  render: (val)->
    # humanize or whatever
    return humanize.numberFormat(val)

module.exports = BasicPanelView

