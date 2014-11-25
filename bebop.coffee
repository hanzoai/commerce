fs   = require 'fs'
path = require 'path'

requisite = 'node_files/.bin/requisite'
stylus    = 'node_files/.bin/stylus'

files =
  checkout:
    js:
      in:  'assets/js/checkout/checkout.coffee'
      out: 'static/js/checkout.js'
    css:
      in:  'assets/css/checkout/checkout.styl'
      out: 'static/css'

  preorder:
    js:
      in:  'assets/js/preorder/preorder.coffee'
      out: 'static/js/preorder.js'
    css:
      in:  'assets/css/preorder/preorder.styl'
      out: 'static/css'

  store:
    js:
      in:  'assets/js/store/store.coffee'
      out: 'static/js/store.js'
    css:
      in:  'assets/css/store/store.styl'
      out: 'static/css'

module.exports =
  cwd: process.cwd() + '/assets'

  forceReload: true

  compilers:
    coffee: (src) ->
      # try to just optimize module changed
      if /^js\/checkout/.test src
        return "#{requisite} #{files.checkout.js.in} -o #{files.checkout.js.out} -g -s"
      if /^js\/preorder/.test src
        return "#{requisite} #{files.preorder.js.in} -o #{files.preorder.js.out} -g -s"
      if /^js\/store/.test src
        return "#{requisite} #{files.store.js.in} -o #{files.store.js.out} -g -s"

      if /^js\//.test src
        # compile everything
        input  = (v.in for k,v of files).join ' '
        output = ('-o ' + v.out for k,v of files).join ' '

        return "#{requisite} #{input} #{output} -g -s"

    styl: (src) ->
      # try to just optimize module changed
      if /^css\/checkout/.test src
        return "#{files.checkout.css.in} -o #{files.checkout.css.out}"
      if /^css\/preorder/.test src
        return "#{files.preorder.css.in} -o #{files.preorder.css.out}"
      if /^css\/store/.test src
        return "#{files.store.css.in} -o #{files.store.css.out}"

      if /^css\//.test src
        # compile everything
        input  = (v.in for k,v of files).join ' '

        return "#{stylus} #{input} -o static/css/"
