fs   = require 'fs'
path = require 'path'

requisite = 'node_modules/.bin/requisite'
stylus    = 'node_modules/.bin/stylus'

module.exports =
  cwd: process.cwd() + '/assets'

  forceReload: true

  compilers:
    coffee: (src) ->
      console.log src
      if /^js\/checkout/.test src
        return "#{requisite} assets/js/checkout/checkout.coffee -o static/js/checkout.js -g -s"
      if /^js\/preorder/.test src
        return "#{requisite} assets/js/preorder/preorder.coffee -o static/js/preorder.js -g -s"
      if /^js\/store/.test src
        return "#{requisite} assets/js/store/store.coffee -o static/js/store.js -g -s"

    styl: (src) ->
      "#{stylus} assets/css/preorder/preorder.styl -o static/css/"
