fs   = require 'fs'
path = require 'path'

requisite = 'node_modules/.bin/requisite -s -g'
stylus    = 'node_modules/.bin/stylus -u autoprefixer-stylus --sourcemap --sourcemap-inline'

files =
  api:
    js:
      in:  'assets/js/api/api.coffee'
      out: 'static/js/api.js'

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

  theme:
    # js:
    #   in:  'assets/js/theme/theme.coffee'
    #   out: 'static/js/theme.js'
    css:
      in:  'assets/css/theme/theme.styl'
      out: 'static/css'

module.exports =
  cwd: process.cwd()

  exclude: [
    /api\/static/
    /checkout\/static/
    /platform\/static/
    /preorder\/static/
    /store\/static/
    /theme\/static/
    /config\/production\/assets/
    /config\/production\/static/
    /config\/sandbox\/assets/
    /config\/sandbox\/static/
    /config\/skully\/assets/
    /config\/skully\/static/
    /config\/staging\/assets/
    /config\/staging\/static/
    /config.json$/
    /\.go$/
    /\.yaml$/
  ]

  compilers:
    coffee: (src) ->
      # try to just optimize module changed
      if /^assets\/js\/checkout/.test src
        return "#{requisite} #{files.checkout.js.in} -o #{files.checkout.js.out}"
      if /^assets\/js\/preorder/.test src
        return "#{requisite} #{files.preorder.js.in} -o #{files.preorder.js.out}"
      if /^assets\/js\/store/.test src
        return "#{requisite} #{files.store.js.in} -o #{files.store.js.out}"
      if /^assets\/js\/api/.test src
        return "#{requisite} #{files.api.js.in} -o #{files.api.js.out}"

      if /^assets\/js\//.test src
        output = []
        input = []
        for _, settings of files
          if settings.js?
            input.push settings.js.in
            output.push "-o #{settings.js.out}"

        return "#{requisite} #{input.join ' '} #{output.join ' '}"

    styl: (src) ->
      # try to just optimize module changed
      if /^assets\/css\/checkout/.test src
        return "#{stylus} #{files.checkout.css.in} -o #{files.checkout.css.out}"
      if /^assets\/css\/preorder/.test src
        return "#{stylus} #{files.preorder.css.in} -o #{files.preorder.css.out}"
      if /^assets\/css\/store/.test src
        return "#{stylus} #{files.store.css.in} -o #{files.store.css.out}"
      if /^assets\/css\/theme/.test src
        return "#{stylus} #{files.theme.css.in} -o #{files.theme.css.out}"

      if /^assets\/css\//.test src
        # compile everything
        input = []
        for _, settings of files
          if settings.css?.in?
            input.push settings.css.in

        return "#{stylus} #{input.join ' '} -o static/css/"
