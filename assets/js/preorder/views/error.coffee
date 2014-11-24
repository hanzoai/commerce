View = require 'mvstar/lib/view'

class ErrorView extends View
  html: '''
  <div class="error-container">
    <a class="error-link">
      <span class="error-message"></span>
    </a>
  </div>
  '''

  bindings:
    message: '.error-message @text'
    link:    '.error-link    @href'

module.exports = ErrorView
