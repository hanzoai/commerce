crowdcontrol = require 'crowdcontrol'

View = crowdcontrol.view.View

class Integration extends View
  tag: 'basic-integration'
  html: ''
  img: window.staticUrl + '/img/integrations/basic.png'
  alt: 'Basic'

  name: 'Basic Integration'
  Set: require './set'

  js: (opts)->
    super
    $(@root).attr('id', 'current-integration').addClass('animated').addClass('fadeIn')

  @register: ()->
    @prototype.Set.prototype.integrations.push(@)
    super

module.exports = Integration
