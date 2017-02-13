Page = require './page'

class MailingList extends Page
  tag: 'page-mailinglist'
  icon: 'fa fa-envelope'
  name: 'Mailing List'
  html: require '../../templates/dash/site/pages/mailinglist.html'

  collection: 'mailinglist'

  js: ()->
    super

    @on 'update', ()->
      requestAnimationFrame ()->
        try
          $code = $('pre code')
          return if $code.html().indexOf('undefined') > -1

          $code.each (i, block)->
            hljs.highlightBlock block
        catch e
          e

MailingList.register()

module.exports = MailingList
