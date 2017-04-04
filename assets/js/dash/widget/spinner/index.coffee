crowdcontrol = require 'crowdcontrol'

View = crowdcontrol.view.View
m = crowdcontrol.utils.mediator

class ModalSpinner extends View
  tag:  'modal-spinner'
  html: '''
    <div class="{ animated: true, fadeIn: isActive(), fadeOut: !isActive(), hide: hide }">'
      <!-- Loader -->
      <div class="loader">
        <div class="sk-folding-cube">
          <div class="sk-cube1 sk-cube"></div>
          <div class="sk-cube2 sk-cube"></div>
          <div class="sk-cube4 sk-cube"></div>
          <div class="sk-cube3 sk-cube"></div>
        </div>
      </div>
    </div>
  '''
  js: ()->
    @active = {}
    @hide = true
    hideId = 0
    m.on 'start-spin', (name = '')=>
      @hide = false
      clearTimeout hideId
      @active[name] = true
      @update()

    m.on 'stop-spin', (name = '')=>
      clearTimeout hideId
      hideId = setTimeout ()=>
        @hide = true
        @update()
      , 1000
      @active[name] = false
      @update()

  isActive:()->
    if @active?
      for name, active of @active
        if active
          return true

    return false

ModalSpinner.register()
