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

  dragstart: (e)->
    @obs.trigger 'dragstart', e, @model
    e.dataTransfer.setData('application/node type', e.target)
    return true

  dragend: (e)->
    @obs.trigger 'dragend', e, @model
    return true

  drag: (e)->
    @obs.trigger 'drag', e, @model
    return true

Drag.register()

class Drop extends View
  tag: 'drop'
  html: '''
    <div
      ondrop="{drop}"
      ondragover="{dragover}"
      ondragenter="{dragenter}"
      ondragleave="{dragleave}">
      <yield></yield>
    </div>
  '''

  drop: (e)->
    @obs.trigger 'drop', e
    return true

  dragover: (e)->
    @obs.trigger 'dragover', e
    e.preventDefault()
    return false

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
