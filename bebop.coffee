fs   = require 'fs'
path = require 'path'

requisite = 'node_modules/.bin/requisite'
stylus = 'node_modules/.bin/stylus'

module.exports =
  cwd: process.cwd() + '/assets/js'

  forceReload: true

  compilers:
    coffee: (src) ->
      if /^checkout/.test src
        return requisite + ' assets/js/checkout/checkout.coffee -o static/js/checkout.js'
      if /^preorder/.test src
        return requisite + ' assets/js/preorder/preorder.coffee -o static/js/preorder.js'
      if /^store/.test src
        return requisite + ' assets/js/store/store.coffee -o static/js/store.js'

    styl: (src) ->
      stylus + ' assets/css/preorder/preorder.styl -o static/css/preorder.css'
