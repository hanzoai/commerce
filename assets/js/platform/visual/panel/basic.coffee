crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events
View = crowdcontrol.view.View

spinFrames = 10

Events.Visual =
  NewData: 'visual-new-data'
  NewDescription: 'visual-new-description'
  NewLabel: 'visual-new-label'

class BasicPanelView extends View
  tag: 'basic-panel'
  label: ''
  description: 'Description'
  html: require '../../templates/backend/visual/panel/template.html'
  events:
    "#{ Events.Visual.NewData }": (model, compareModel)->
      @loadData model, compareModel
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
      @comparePercent = ((@model - @compareModel) / @compareModel).toFixed(3) * 100

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

        if frames > 0
          frames--
          spin()
    spin()

  render: (val)->
    # humanize or whatever
    return val

module.exports = BasicPanelView

