fs   = require 'fs'
path = require 'path'

requisite        = 'node_modules/.bin/requisite'
stylus           = 'node_modules/.bin/stylus'

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
  cwd: process.cwd()

  exclude: [
    /config\/production\/static/
  ]

  compilers:
    coffee: (src) ->
      # try to just optimize module changed
      if /^assets\/js\/checkout/.test src
        return "#{requisite} #{files.checkout.js.in} -o #{files.checkout.js.out} -g -s"
      if /^assets\/js\/preorder/.test src
        return "#{requisite} #{files.preorder.js.in} -o #{files.preorder.js.out} -g -s"
      if /^assets\/js\/store/.test src
        return "#{requisite} #{files.store.js.in} -o #{files.store.js.out} -g -s"

      if /^assets\/js\//.test src
        # compile everything
        output = []
        input = []
        for _, settings of files
          if settings.js?
            input.push settings.js.in
            output.push settings.js.out

        return "#{requisite} #{input} #{output} -g -s"

    styl: (src) ->
      # try to just optimize module changed
      if /^assets\/css\/checkout/.test src
        return "#{stylus} #{files.checkout.css.in} -o #{files.checkout.css.out}"
      if /^assets\/css\/preorder/.test src
        return "#{stylus} #{files.preorder.css.in} -o #{files.preorder.css.out}"
      if /^assets\/css\/store/.test src
        return "#{stylus} #{files.store.css.in} -o #{files.store.css.out}"

      if /^assets\/css\//.test src
        # compile everything
        input = []
        for _, settings of files
          if settings.css?.in?
            input.push settings.css.in

        return "#{stylus} #{input} -o static/css/"
