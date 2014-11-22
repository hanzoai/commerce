fs   = require 'fs'
path = require 'path'

module.exports =
  cwd: process.cwd() + '/assets/js'

  port: 3001

  compilers:
    coffee: (src) ->
      if /^checkout/.test src
        return 'requisite assets/js/checkout/checkout.coffee -o static/js/checkout.js'
      if /^preorder/.test src
        return 'requisite assets/js/preorder/preorder.coffee -o static/js/preorder.js'
      if /^store/.test src
        return 'requisite assets/js/store/store.coffee -o static/js/store.js'

    styl: (src) ->
      'stylus assets/css/preorder/preorder.styl -o static/css/preorder.css'
