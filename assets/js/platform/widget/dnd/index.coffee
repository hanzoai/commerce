riot = require 'riot'

crowdcontrol = require 'crowdcontrol'
Events = crowdcontrol.Events
View = crowdcontrol.view.View

class Drag extends View
  tag: 'drag'
  html: '''
    <div draggable="true"
      ondragstart="{dragstart}"
      ondragend="{dragend}"
      ondrag="{drag}">
      <yield></yield>
    </div>
  '''

  js: ()->
    super
    @model = @parent

  dragstart: (e)->
    @obs.trigger 'dragstart', e
    return true

  dragend: (e)->
    @obs.trigger 'dragend', e
    return true

  drag: (e)->
    @obs.trigger 'drag', e
    return true

Drag.register()

class Drop extends View
  tag: 'drop'
  html: '''
    <div
      ondrop="{drop}"
      ondragover="{dragover}"
      ondragenter="{dragenter}"
      ondragleave="{dragleave}"
      <yield></yield>
    </div>
  '''

  drop: (e)->
    @obs.trigger 'drop', e
    return true

  dragover: (e)->
    @obs.trigger 'dragover', e
    return true

  dragenter: (e)->
    @obs.trigger 'dragenter', e
    return true

  dragleave: (e)->
    @obs.trigger 'dragleave', e
    return true

Drop.register()

module.export =
  Drag: Drag
  Drop: Drop
