fs   = require 'fs'
path = require 'path'

requisite = 'node_modules/.bin/requisite'
stylus    = 'node_modules/.bin/stylus'

modules =
  checkout:
    in:  'assets/js/checkout/checkout.coffee'
    out: 'static/js/checkout.js'

  preorder:
    in:  'assets/js/preorder/preorder.coffee'
    out: 'static/js/preorder.js'

  store:
    in:  'assets/js/store/store.coffee'
    out: 'static/js/store.js'

module.exports =
  cwd: process.cwd() + '/assets'

  forceReload: true

  compilers:
    coffee: (src) ->
      # try to just optimize module changed
      if /^js\/checkout/.test src
        return "#{requisite} #{modules.checkout.in} -o #{modules.checkout.out} -g -s"
      if /^js\/preorder/.test src
        return "#{requisite} #{modules.preorder.in} -o #{modules.preorder.out} -g -s"
      if /^js\/store/.test src
        return "#{requisite} #{modules.store.in} -o #{modules.store.out} -g -s"

      if /^js\//.test src
        # compile everything
        input  = (v.in for k,v of modules).join ' '
        output = ('-o ' + v.out for k,v of modules).join ' '

        console.log input
        console.log output

        return "#{requisite} #{input} #{output} -g -s"

    styl: (src) ->
      "#{stylus} assets/css/preorder/preorder.styl -o static/css/"
