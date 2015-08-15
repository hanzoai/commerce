crowdcontrol = require 'crowdcontrol'

FormView = crowdcontrol.view.form.FormView

class Integration extends FormView
  tag: 'basic-integration'
  type: 'basic-integration'
  html: ''
  instructions: 'Information on what to expect from the integration'
  img: '/img/integrations/basic.png'
  text: ''#'Basic Integration'
  alt: 'Basic'

  name: 'Basic Integration'

  js: (opts)->
    super
    $(@root).attr('id', 'current-integration').addClass('animated').addClass('fadeIn')

  @data: ()->
    return {
      integration: @
      src: @src()
      text: @prototype.text
      alt: @prototype.alt
    }

  @src: ()->
    window.staticUrl + @prototype.img

module.exports = Integration
