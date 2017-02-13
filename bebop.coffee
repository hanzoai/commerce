fs   = require 'fs'
path = require 'path'

coffee    = 'node_modules/.bin/coffee'
requisite = 'node_modules/.bin/requisite -g'
stylus    = 'node_modules/.bin/stylus -u autoprefixer-stylus --sourcemap --sourcemap-inline'

files =
  dash:
    js:
      in:  'assets/js/dash/dash.coffee'
      out: 'static/js/dash.js'
    css:
      in:  'assets/css/dash/dash.styl'
      out: 'static/css'

  api:
    js:
      in:  'assets/js/api/api.coffee'
      out: 'static/js/api.js'

  analyticsNative:
    js:
      in:  'assets/js/analytics/native.coffee'
      out: 'static/js/analytics/native.js'

  mailinglist:
    js:
      in:  'assets/js/api/mailinglist.coffee'
      out: 'static/js'

  theme:
    css:
      in:  'assets/css/theme/theme.styl'
      out: 'static/css'

module.exports =
  cwd: process.cwd()

  host: 'localhost'
  port: 8090

  exclude: [
    /api\/static/
    /config.json$/
    /config\/production\/assets/
    /config\/production\/static/
    /config\/sandbox\/assets/
    /config\/sandbox\/static/
    /config\/staging\/assets/
    /config\/staging\/static/
    /dash\/static/
    /dash\/templates/
    /static\/vendor\/plugins/
    /\.go$/
    /\.test$/
    /\.yaml$/
  ]

  compilers:
    coffee: (src) ->
      # try to just optimize module changed
      if /^assets\/js\/dash/.test src
        return "#{requisite} #{files.dash.js.in} -o #{files.dash.js.out}"
      if /^assets\/js\/analytics\/native/.test src
        return "#{requisite} #{files.analyticsNative.js.in} -o #{files.analyticsNative.js.out}"
      if /^assets\/js\/api/.test src
        if /mailinglist/.test src
          return "#{coffee} -bc -o #{files.mailinglist.js.out} #{files.mailinglist.js.in}"
        else
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
      if /^assets\/css\/dash/.test src
        return "#{stylus} #{files.dash.css.in} -o #{files.dash.css.out}"
      if /^assets\/css\/theme/.test src
        return "#{stylus} #{files.theme.css.in} -o #{files.theme.css.out}"

      if /^assets\/css\//.test src
        # compile everything
        input = []
        for _, settings of files
          if settings.css?.in?
            input.push settings.css.in

        return "#{stylus} #{input.join ' '} -o static/css/"
