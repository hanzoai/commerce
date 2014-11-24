View = require 'mvstar/lib/view'

class ErrorView extends View
  template: '#error-template'

  bindings:
    message: '.error-message @text'
    link:    '.error-link    @href'

module.exports = ErrorView
