_ = require 'underscore'

crowdcontrol = require 'crowdcontrol'

View = crowdcontrol.view.View

class Integration extends View
  tag: 'basic-integration'
  img: window.staticUrl + '/img/integrations/basic.png'

  name: 'Basic Integration'
  Set: require './set'

  js: (opts)->
    super
    $(@root).attr('id', 'current-integration').addClass('animated').addClass('fadeIn')

  @register: ()->
    @prototype.Set.integrations.push(@)
    super

module.exports = Integration
